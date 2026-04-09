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

// relay bridges the Trigger channel to the Worker channel for a single entry.
//
// Two goroutines run inside relay.run:
//
//	g1 (intake):  reads events from entry.eventCh and appends them to the
//	              in-memory queue.
//	g2 (dispatch): delegates to the per-policy dispatcher, which dequeues
//	              events and sends workItems to entry.workCh.
//
// g1 signals g2 via notifyCh (capacity 1) each time it enqueues an event.
// g1 closes notifyCh on exit; dispatchers detect this (ok=false) to know
// the trigger is exhausted and no more events will ever arrive.
//
// forceCh is closed by forceStop to signal dispatchers to exit immediately
// without draining the queue.
type relay struct {
	entry      *entry
	mu         sync.Mutex
	queue      []types.Event
	notifyCh   chan struct{} // g1 → g2 signal (cap 1); closed by g1 on exit
	forceCh    chan struct{} // closed by forceStop; signals immediate exit
	dispatcher dispatcher
}

func newRelay(e *entry) *relay {
	return &relay{
		entry:      e,
		notifyCh:   make(chan struct{}, 1),
		forceCh:    make(chan struct{}),
		dispatcher: newPolicyDispatcher(e.policy),
	}
}

// run starts g1 (as a goroutine) and runs g2 (dispatcher.run) inline.
// It closes entry.workCh on return to signal the worker.
func (r *relay) run(ctx context.Context) {
	defer close(r.entry.workCh)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.intake(ctx)
	}()

	r.dispatcher.run(ctx, r, r.entry.workCh) // blocks until g2 is done
	wg.Wait()                                // wait for g1 to exit
}

// intake (g1) reads events from entry.eventCh, appends them to the queue, and
// pings g2 via notifyCh. It exits when eventCh is closed or ctx is cancelled,
// closing notifyCh to signal g2 that no more events will arrive.
func (r *relay) intake(ctx context.Context) {
	defer close(r.notifyCh)

	for {
		select {
		case ev, ok := <-r.entry.eventCh:
			if !ok {
				return
			}
			r.mu.Lock()
			if len(r.queue) >= maxQueueSize {
				// Drop the oldest event to make room, preserving recent state.
				r.queue = r.queue[1:]
				log.Warn().Str("job", r.entry.job.Name()).
					Msg("relay queue full, dropping oldest event")
			}
			r.queue = append(r.queue, ev)
			r.mu.Unlock()
			// Non-blocking ping: if notifyCh already has a pending signal,
			// g2 will pick up the newly added item on its next wake.
			select {
			case r.notifyCh <- struct{}{}:
			default:
			}
		case <-ctx.Done():
			// Graceful shutdown: drain any events the trigger already emitted
			// into eventCh so they reach the relay queue before g2 exits.
			// The trigger goroutine closes eventCh when its Emit returns
			// (which it does promptly on ctx cancellation), so this loop is
			// bounded.
			for ev := range r.entry.eventCh {
				r.mu.Lock()
				if len(r.queue) >= maxQueueSize {
					r.queue = r.queue[1:]
					log.Warn().Str("job", r.entry.job.Name()).
						Msg("relay queue full, dropping oldest event")
				}
				r.queue = append(r.queue, ev)
				r.mu.Unlock()
				select {
				case r.notifyCh <- struct{}{}:
				default:
				}
			}
			return
		}
	}
}

// forceStop clears the queue, cancels any in-flight worker job, and signals
// all dispatchers to exit immediately without draining.
// Called by Scheduler.Stop(true) before cancelling the run context.
func (r *relay) forceStop() {
	r.mu.Lock()
	r.queue = r.queue[:0]
	r.mu.Unlock()
	r.dispatcher.cancel()
	close(r.forceCh)
}
