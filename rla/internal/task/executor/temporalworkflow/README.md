# Temporal Workflow Guide

This guide demonstrates how to create a new operation driven by Temporal workflow in the RLA system.

## Table of Contents
- [Overview](#overview)
- [Architecture](#architecture)
- [Creating a New Operation](#creating-a-new-operation)
  - [Step 1: Define Activities](#step-1-define-activities)
  - [Step 2: Register Activities](#step-2-register-activities)
  - [Step 3: Create Workflow](#step-3-create-workflow)
  - [Step 4: Register Workflow](#step-4-register-workflow)
  - [Step 5: Add Manager Method](#step-5-add-manager-method)
- [Complete Example](#complete-example)
- [Best Practices](#best-practices)

## Overview

The RLA temporal workflow system provides a reliable and scalable way to orchestrate long-running operations across distributed components. It consists of three main layers:

1. **Manager**: Entry point that starts workflows and manages temporal clients
2. **Workflows**: Orchestrate activities and define execution logic
3. **Activities**: Actual work units that interact with component managers

## Architecture

```
┌─────────────────┐
│     Manager     │  - Starts workers
│                 │  - Manages temporal clients
└────────┬────────┘
         │
         │ ExecuteWorkflow
         ▼
┌─────────────────┐
│    Workflow     │  - Orchestrates activities
│                 │  - Defines execution sequence
└────────┬────────┘
         │
         │ ExecuteActivity (parallel/sequential)
         ▼
┌─────────────────┐
│   Activities    │  - Execute actual operations
│                 │  - Interact with components
└─────────────────┘
```

## Creating a New Operation

Let's create a new operation called "HealthCheck" as an example. This operation will check the health of all components in a rack.

### Step 1: Define Activities

Activities are the basic units of work. They should be idempotent and handle retries gracefully.

**File: `activity/activity.go`**

Add a new activity function:

```go
// HealthCheck checks the health status of a component
func HealthCheck(
	ctx context.Context,
	req common.HealthCheckRequest,
) (common.HealthStatus, error) {
	cm, err := validAndGetComponentManager(req.ComponentInfo)
	if err != nil {
		return common.HealthStatusUnknown, err
	}

	return cm.HealthCheck(ctx, req)
}
```

**Key Points:**
- Activities take `context.Context` as the first parameter
- Use request/response structs from `common` package
- Activities should validate inputs and handle errors gracefully
- Activities are automatically retried by Temporal based on retry policy

### Step 2: Register Activities

Add your activity to the registry so it can be discovered by workers.

**File: `activity/activity.go`**

Update the `GetAllActivities()` function:

```go
func GetAllActivities() []any {
	return []any{
		InjectExpectation,
		PowerControl,
		FirmwareControl,
		Status,
		FirmwareVersion,
		PowerStatus,
		HealthCheck,  // Add your new activity here
	}
}
```

### Step 3: Create Workflow

Workflows orchestrate activities. They define the execution logic, sequencing, and error handling.

**File: `workflow/healthcheck.go`** (create new file)

```go
package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/nvidia/bare-metal-manager-rest/rla/internal/componentmanager/common"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/component"
	"github.com/nvidia/bare-metal-manager-rest/rla/pkg/inventoryobjects/rack"
)

const (
	HealthCheckWorkflowExecutionTimeout = 30 * time.Minute
	HealthCheckWorkflowName             = "HealthCheck"
)

var (
	healthCheckActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    1 * time.Minute,
			BackoffCoefficient: 2,
		},
	}
)

// HealthCheck workflow checks the health of all components in a rack
func HealthCheck(
	ctx workflow.Context,
	rack *rack.Rack,
) (map[string]common.HealthStatus, error) {
	if rack == nil {
		return nil, fmt.Errorf("rack is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, healthCheckActivityOptions)

	// Execute health checks for all components in parallel
	futures := make(map[string]workflow.Future)
	for _, c := range rack.Components {
		componentName := c.Info.Name
		futures[componentName] = healthCheckComponent(ctx, &c)
		log.Debug().Msgf("health check for component %s started", componentName)
	}

	// Collect results
	results := make(map[string]common.HealthStatus)
	errs := make([]error, 0)

	for componentName, f := range futures {
		var status common.HealthStatus
		err := f.Get(ctx, &status)
		if err != nil {
			log.Error().Msgf("health check for component %s failed: %v", componentName, err)
			errs = append(errs, fmt.Errorf("%s: %w", componentName, err))
			status = common.HealthStatusUnknown
		}
		results[componentName] = status
		log.Debug().Msgf("health check for component %s completed: %s", componentName, status)
	}

	// Return results even if some checks failed
	if len(errs) > 0 {
		return results, combineErrors(errs)
	}

	return results, nil
}

func healthCheckComponent(
	ctx workflow.Context,
	comp *component.Component,
) workflow.Future {
	req := common.HealthCheckRequest{
		ComponentInfo: common.ComponentInfo{
			Type:       comp.Type,
			DeviceInfo: comp.Info,
		},
	}

	return workflow.ExecuteActivity(ctx, "HealthCheck", req)
}

func combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("multiple errors occurred: ")
	for i, err := range errs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(err.Error())
	}

	return fmt.Errorf(sb.String())
}
```

**Key Points:**
- Workflows must be deterministic (no random values, no direct external calls)
- Use `workflow.Context`, not `context.Context`
- Define activity options (timeouts, retry policies)
- Use `workflow.ExecuteActivity()` to call activities
- Activities can be executed in parallel using futures
- Return meaningful results and errors

### Step 4: Register Workflow

Add your workflow to the registry so it can be discovered by workers.

**File: `workflow/workflow.go`**

```go
func GetAllWorkflows() []any {
	return []any{
		PowerControl,
		HealthCheck,  // Add your new workflow here
	}
}
```

### Step 5: Add Manager Method

Create a method in the Manager to start your workflow. This is the entry point that external callers use.

**File: `manager/manager.go`**

```go
// HealthCheck executes a health check workflow for a rack
func (m *Manager) HealthCheck(ctx context.Context, rack *rack.Rack) (map[string]common.HealthStatus, error) {
	workflowOptions := temporalclient.StartWorkflowOptions{
		TaskQueue:                WorkflowQueue,
		ID:                       fmt.Sprintf("health-check-%s", rack.Info.Name),
		WorkflowExecutionTimeout: workflow.HealthCheckWorkflowExecutionTimeout,
	}

	r, err := m.publisherClient.Client().ExecuteWorkflow(
		ctx,
		workflowOptions,
		workflow.HealthCheckWorkflowName,
		rack,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to execute health check workflow: %v", err)
	}

	wid := r.GetID()
	log.Info().Msgf(
		"Health check workflow started [wid: %s, rack: %s]",
		wid,
		rack.Info.Name,
	)

	// Wait for the workflow to complete and get results
	var results map[string]common.HealthStatus
	if err := r.Get(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
```

**Key Points:**
- Use unique workflow IDs to prevent duplicates
- Set appropriate timeouts
- Pass workflow name and parameters
- Handle both synchronous (blocking) and asynchronous patterns
- Log workflow execution for observability

## Complete Example

Here's the complete flow for the HealthCheck operation:

### 1. Request/Response Structs (common package)

```go
// In internal/componentmanager/common/requests.go
type HealthCheckRequest struct {
	ComponentInfo ComponentInfo
}

// In internal/componentmanager/common/status.go
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)
```

### 2. Component Manager Implementation

```go
// ComponentManager interface should include:
type ComponentManager interface {
	// ... other methods ...
	HealthCheck(ctx context.Context, req HealthCheckRequest) (HealthStatus, error)
}
```

### 3. Usage Example

```go
// In your service layer
func (s *Service) CheckRackHealth(ctx context.Context, rackName string) error {
	// Get rack from database
	rack, err := s.rackManager.GetRack(ctx, rackName)
	if err != nil {
		return err
	}

	// Execute health check workflow
	results, err := s.temporalManager.HealthCheck(ctx, rack)
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	// Process results
	for component, status := range results {
		log.Info().Msgf("Component %s: %s", component, status)
	}

	return nil
}
```

## Best Practices

### Workflow Design

1. **Keep workflows deterministic**: Don't use random numbers, current time, or external calls directly
2. **Use activities for non-deterministic work**: All I/O, API calls, and side effects go in activities
3. **Handle errors gracefully**: Workflows should be resilient to partial failures
4. **Use meaningful workflow IDs**: Makes it easier to track and debug
5. **Set appropriate timeouts**: Balance between giving enough time and detecting hung workflows

### Activity Design

1. **Make activities idempotent**: They may be retried multiple times
2. **Keep activities focused**: Each activity should do one thing well
3. **Use structured logging**: Include context like component name, operation type
4. **Validate inputs**: Check all parameters before executing
5. **Return detailed errors**: Help with debugging when things fail

### Performance Considerations

1. **Parallelize when possible**: Use futures to execute independent activities concurrently
2. **Batch operations**: Group related operations to reduce overhead
3. **Use appropriate timeouts**: Don't make them too short or too long
4. **Monitor workflow execution**: Track duration and success rates

### Error Handling

1. **Use retry policies**: Configure appropriate retry behavior for activities
2. **Collect partial results**: Don't fail the entire workflow if one component fails
3. **Provide context in errors**: Include which component/operation failed
4. **Log at appropriate levels**: Info for normal flow, Error for actual problems

### Testing

1. **Test activities independently**: Unit test each activity function
2. **Test workflows with mock activities**: Use Temporal's test framework
3. **Test error scenarios**: Verify retry and error handling logic
4. **Test Manager methods**: Integration tests for the full flow

### Observability

1. **Log workflow starts and completions**: Include workflow ID and parameters
2. **Log activity executions**: Track when activities start and complete
3. **Use structured logging**: Makes it easier to search and analyze
4. **Add metrics**: Track success rates, durations, retry counts
5. **Use Temporal Web UI**: Monitor workflows in real-time

## Workflow Patterns

### Sequential Execution

Execute activities one after another:

```go
func SequentialWorkflow(ctx workflow.Context, rack *rack.Rack) error {
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Step 1
	if err := workflow.ExecuteActivity(ctx, "Activity1", arg1).Get(ctx, nil); err != nil {
		return err
	}

	// Step 2
	if err := workflow.ExecuteActivity(ctx, "Activity2", arg2).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}
```

### Parallel Execution

Execute multiple activities at once:

```go
func ParallelWorkflow(ctx workflow.Context, rack *rack.Rack) error {
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	futures := make([]workflow.Future, 0)
	for _, comp := range rack.Components {
		f := workflow.ExecuteActivity(ctx, "Activity", comp)
		futures = append(futures, f)
	}

	// Wait for all to complete
	for _, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			return err
		}
	}

	return nil
}
```

### Conditional Execution

Execute activities based on conditions:

```go
func ConditionalWorkflow(ctx workflow.Context, rack *rack.Rack, mode string) error {
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	switch mode {
	case "full":
		return workflow.ExecuteActivity(ctx, "FullCheck", rack).Get(ctx, nil)
	case "quick":
		return workflow.ExecuteActivity(ctx, "QuickCheck", rack).Get(ctx, nil)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}
```

### Sequenced Groups

Execute groups of activities in sequence, with parallel execution within each group:

```go
func SequencedGroupsWorkflow(ctx workflow.Context, rack *rack.Rack) error {
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Group 1: Power on power shelves (parallel)
	group1 := make([]workflow.Future, 0)
	for _, ps := range powerShelves {
		f := workflow.ExecuteActivity(ctx, "PowerOn", ps)
		group1 = append(group1, f)
	}
	for _, f := range group1 {
		if err := f.Get(ctx, nil); err != nil {
			return err
		}
	}

	// Group 2: Power on switches (parallel)
	group2 := make([]workflow.Future, 0)
	for _, sw := range switches {
		f := workflow.ExecuteActivity(ctx, "PowerOn", sw)
		group2 = append(group2, f)
	}
	for _, f := range group2 {
		if err := f.Get(ctx, nil); err != nil {
			return err
		}
	}

	return nil
}
```

## References

- [Temporal Documentation](https://docs.temporal.io/)
- [Temporal Go SDK](https://github.com/temporalio/sdk-go)
- Existing implementations:
  - `workflow/powercontrol.go` - Power control workflow
  - `activity/activity.go` - Activity implementations
  - `manager/manager.go` - Manager with workflow execution

