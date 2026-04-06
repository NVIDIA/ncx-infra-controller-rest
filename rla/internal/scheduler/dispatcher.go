/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package scheduler

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/NVIDIA/ncx-infra-controller-rest/rla/internal/scheduler/types"
)

// dispatcher is the per-policy component of relay g2.
// It decides when and how events are sent from the relay queue to the worker.
//
// Every implementation maintains cancelCurrent so that relay.forceStop can
// abort the in-flight job regardless of which policy is active.
type dispatcher interface {
	// run is the g2 event loop. It reads from r.notifyCh, dequeues events
	// from r, and sends workItems to workCh. It blocks until it determines
	// it should exit (ctx cancelled, or all events flushed for QueueAll).
	run(ctx context.Context, r *relay, workCh chan<- workItem)

	// cancel cancels the currently running worker job, if any.
	// Called by relay.forceStop to abort in-flight work immediately.
	cancel()
}

// newPolicyDispatcher returns the dispatcher implementation for the given policy.
func newPolicyDispatcher(p types.Policy) dispatcher {
	switch p {
	case types.Queue:
		return &queueDispatcher{}
	case types.QueueAll:
		return &queueAllDispatcher{}
	case types.Replace:
		return &replaceDispatcher{}
	default: // types.Skip
		return &skipDispatcher{}
	}
}

// --- shared helpers ---

// dispatchBase holds the cancelCurrent field shared by all implementations.
type dispatchBase struct {
	mu            sync.Mutex
	cancelCurrent context.CancelFunc
}

func (b *dispatchBase) setCancel(cancel context.CancelFunc) {
	b.mu.Lock()
	b.cancelCurrent = cancel
	b.mu.Unlock()
}

func (b *dispatchBase) cancel() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancelCurrent != nil {
		b.cancelCurrent()
	}
}

// --- skipDispatcher ---

// skipDispatcher drops the event when the worker is busy (non-blocking send).
type skipDispatcher struct {
	dispatchBase
}

func (d *skipDispatcher) run(
	ctx context.Context, r *relay, workCh chan<- workItem,
) {
	for {
		select {
		case _, ok := <-r.notifyCh:
			if !ok {
				return // trigger exhausted
			}
		case <-r.forceCh:
			return // force stop: queue already cleared by relay.forceStop
		case <-ctx.Done():
			return
		}

		r.mu.Lock()
		if len(r.queue) == 0 {
			r.mu.Unlock()
			continue
		}

		// Dispatch the oldest one and clear all the others in the queue.
		ev := r.queue[0]
		r.queue = r.queue[:0]
		r.mu.Unlock()

		// Create the job context before the select but only register
		// its cancel with dispatchBase (for forceStop) if the send
		// succeeds. On drop, call jobCancel directly to remove the
		// child from the parent's internal children map; the running
		// job is left untouched.
		jobCtx, jobCancel := context.WithCancel(ctx)
		select {
		case workCh <- workItem{ctx: jobCtx, ev: ev}:
			d.setCancel(jobCancel)
		default:
			jobCancel() // release parent registration; no job was started
			log.Debug().Str("job", r.entry.job.Name()).
				Msg("skip: worker busy, dropping event")
		}
	}
}

// --- queueDispatcher ---

// queueDispatcher keeps only the latest pending event; blocks until the worker
// accepts it. Earlier events in the queue are discarded.
type queueDispatcher struct {
	dispatchBase
}

func (d *queueDispatcher) run(
	ctx context.Context, r *relay, workCh chan<- workItem,
) {
	for {
		// Phase 1: wait for a notification that the queue has a new event.
		select {
		case _, ok := <-r.notifyCh:
			if !ok {
				return // trigger exhausted
			}
		case <-r.forceCh:
			return // force stop: queue already cleared by relay.forceStop
		case <-ctx.Done():
			return
		}

		// Dequeue the latest event and discard earlier ones.
		r.mu.Lock()
		if len(r.queue) == 0 {
			r.mu.Unlock()
			continue
		}
		ev := r.queue[len(r.queue)-1]
		r.queue = r.queue[:0]
		r.mu.Unlock()

		// Phase 2: block until the worker accepts ev. If a newer event
		// arrives while waiting, update ev to the latest and keep
		// blocking — the worker always receives the most recent event
		// available when it next becomes free.
	deliver:
		for {
			jobCtx, jobCancel := context.WithCancel(ctx)
			select {
			case workCh <- workItem{ctx: jobCtx, ev: ev}:
				d.setCancel(jobCancel)
				break deliver
			case _, ok := <-r.notifyCh:
				jobCancel()
				// A newer event arrived; update ev to the latest in the
				// queue, then retry delivery.
				r.mu.Lock()
				if len(r.queue) > 0 {
					ev = r.queue[len(r.queue)-1]
					r.queue = r.queue[:0]
				}
				r.mu.Unlock()
				if !ok {
					return // trigger exhausted while waiting
				}
			case <-r.forceCh:
				jobCancel()
				return
			case <-ctx.Done():
				jobCancel()
				return
			}
		}
	}
}

// --- queueAllDispatcher ---

// queueAllDispatcher delivers every event in FIFO order. On graceful shutdown
// it flushes any remaining queued events before exiting.
type queueAllDispatcher struct {
	dispatchBase
}

func (d *queueAllDispatcher) run(
	ctx context.Context, r *relay, workCh chan<- workItem,
) {
	for {
		// Snapshot and clear the entire queue under a single lock,
		// then deliver events one-by-one without holding the mutex.
		// This minimises lock contention with relay g1.
		//
		// Use context.Background() — not ctx (the scheduler run context) —
		// so that Stop(false) cancelling the run context does not
		// immediately invalidate the job contexts of items being delivered
		// here. Force-stop still works: relay.forceStop calls
		// dispatcher.cancel() which cancels the current job's context
		// directly via dispatchBase.cancelCurrent.
		d.deliver(context.Background(), r, workCh)

		// Queue empty: wait for more events or shutdown.
		select {
		case _, ok := <-r.notifyCh:
			if !ok {
				// g1 has exited: no more events will be produced.
				// Flush anything added between our last drain and g1's exit.
				// Use context.Background() so the flush is not aborted by
				// the already-cancelled scheduler context.
				d.deliver(context.Background(), r, workCh)
				return
			}
			// More items enqueued; loop back to drain.
		case <-r.forceCh:
			return // force stop: skip drain, queue already cleared
		case <-ctx.Done():
			// Graceful shutdown: flush remaining items.
			d.deliver(context.Background(), r, workCh)
			return
		}
	}
}

// deliver snapshots and clears r.queue under a single lock, then sends each
// event to the worker one-by-one using parentCtx as the job context parent.
func (d *queueAllDispatcher) deliver(
	parentCtx context.Context, r *relay, workCh chan<- workItem,
) {
	r.mu.Lock()
	batch := make([]types.Event, len(r.queue))
	copy(batch, r.queue)
	r.queue = r.queue[:0]
	r.mu.Unlock()

	for _, ev := range batch {
		jobCtx, jobCancel := context.WithCancel(parentCtx)
		select {
		case workCh <- workItem{ctx: jobCtx, ev: ev}:
			// Register cancel only after successful delivery so that
			// forceStop always targets an actual running job.
			d.setCancel(jobCancel)
		case <-r.forceCh:
			// Force stop fired while blocked on a send; discard remaining
			// batch and release the unsent context.
			jobCancel()
			return
		}
	}
}

// --- replaceDispatcher ---

// replaceDispatcher cancels the current job and starts a new one on each event.
type replaceDispatcher struct {
	dispatchBase
}

func (d *replaceDispatcher) run(
	ctx context.Context, r *relay, workCh chan<- workItem,
) {
	for {
		select {
		case _, ok := <-r.notifyCh:
			if !ok {
				return // trigger exhausted
			}
		case <-r.forceCh:
			return // force stop: queue already cleared by relay.forceStop
		case <-ctx.Done():
			return
		}

		r.mu.Lock()
		if len(r.queue) == 0 {
			r.mu.Unlock()
			continue
		}
		// Take the latest event and drop everything else: earlier events
		// are superseded and no longer relevant.
		ev := r.queue[len(r.queue)-1]
		r.queue = r.queue[:0]
		r.mu.Unlock()

		// Cancel the running job before dispatching the replacement.
		d.cancel()

		jobCtx, jobCancel := context.WithCancel(ctx)
		select {
		case workCh <- workItem{ctx: jobCtx, ev: ev}:
			// Register cancel only after successful delivery so that
			// forceStop always targets an actual running job.
			d.setCancel(jobCancel)
		case <-r.forceCh:
			jobCancel()
			return
		case <-ctx.Done():
			jobCancel()
			return
		}
	}
}
