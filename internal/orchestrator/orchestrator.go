// Package orchestrator executes workflows step by step.
// It is the coordination layer — it drives capability execution,
// invokes the agent for reasoning steps, and manages task spawning.
// Workflows define intent. The orchestrator executes it.
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/dominionthedev/splash/internal/agent"
	"github.com/dominionthedev/splash/internal/capability"
	"github.com/dominionthedev/splash/internal/model"
	"github.com/dominionthedev/splash/internal/storage"
	"github.com/dominionthedev/splash/internal/workflow"
)

// Orchestrator executes workflows.
type Orchestrator struct {
	caps  *capability.Registry
	model model.Model
	store *storage.Store
	log   *log.Logger
}

// New creates an Orchestrator.
func New(caps *capability.Registry, m model.Model, store *storage.Store, logger *log.Logger) *Orchestrator {
	return &Orchestrator{
		caps:  caps,
		model: m,
		store: store,
		log:   logger,
	}
}

// Run executes a workflow from start to finish.
// Steps are executed in order. Reason steps invoke the agent.
// Execute steps run the declared capability.
// Task steps spawn concurrent sub-tasks.
func (o *Orchestrator) Run(ctx context.Context, wf *workflow.Workflow) *workflow.RunResult {
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	start := time.Now()

	result := &workflow.RunResult{WorkflowName: wf.Name}

	o.log.Info("workflow started", "name", wf.Name, "steps", len(wf.Steps))

	// One agent per run, locked to this workflow.
	ag := agent.New(o.model, o.caps, o.store, wf)

	// stepContext accumulates outputs for the agent to reason about.
	var stepContext strings.Builder

	for _, step := range wf.Steps {
		o.log.Info("step", "name", step.Name, "kind", step.Kind)

		sr, err := o.execStep(ctx, wf, step, ag, stepContext.String())
		if err != nil {
			sr = &workflow.StepResult{StepName: step.Name, Error: err}
			result.Steps = append(result.Steps, sr)
			result.Error = err
			o.log.Error("step failed", "name", step.Name, "err", err)

			// Record failure in knowledge.
			o.store.RecordKnowledge(wf.Name, "failure",
				fmt.Sprintf("step=%s failed: %v", step.Name, err),
				[]string{step.Name, wf.Name, "failure"})
			break
		}

		result.Steps = append(result.Steps, sr)

		// Feed this step's output into the running context.
		if sr.Output != "" {
			fmt.Fprintf(&stepContext, "[%s] %s\n", step.Name, sr.Output)
		}

		if sr.Artifact != nil {
			result.Artifacts = append(result.Artifacts, sr.Artifact)
			_ = o.store.SaveArtifact(wf.Name, runID, sr.Artifact.Name, sr.Artifact.Kind, sr.Artifact.Content)
		}
	}

	success := result.Error == nil
	o.store.RecordRun(storage.RunRecord{
		RunID:      runID,
		Workflow:   wf.Name,
		Success:    success,
		Steps:      len(result.Steps),
		StartedAt:  start,
		FinishedAt: time.Now(),
		Error: func() string {
			if result.Error != nil {
				return result.Error.Error()
			}
			return ""
		}(),
	})

	if success {
		o.log.Info("workflow completed", "name", wf.Name, "steps", len(result.Steps))
	}
	return result
}

// execStep dispatches a single step to the correct handler.
func (o *Orchestrator) execStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	ag *agent.Agent,
	stepContext string,
) (*workflow.StepResult, error) {

	switch step.Kind {

	case workflow.StepExecute:
		return o.execCapability(ctx, wf, step)

	case workflow.StepReason:
		return o.execReason(ctx, step, ag, stepContext)

	case workflow.StepTask:
		return o.execTask(ctx, wf, step, ag, stepContext)

	default:
		return nil, fmt.Errorf("orchestrator: unknown step kind %q", step.Kind)
	}
}

// execCapability runs a declared capability.
// Enforces scope — only capabilities listed in the workflow's scope are allowed.
func (o *Orchestrator) execCapability(ctx context.Context, wf *workflow.Workflow, step *workflow.Step) (*workflow.StepResult, error) {
	// Scope enforcement — hard block.
	if !wf.Scope.AllowsCapability(step.Capability) {
		return nil, fmt.Errorf("orchestrator: capability %q not declared in workflow scope", step.Capability)
	}

	result := o.caps.Execute(ctx, step.Capability, capability.Input(step.Params))
	if result.Error != nil {
		// Retry logic.
		for attempt := 1; attempt <= step.Retry; attempt++ {
			o.log.Warn("retrying", "step", step.Name, "attempt", attempt)
			result = o.caps.Execute(ctx, step.Capability, capability.Input(step.Params))
			if result.Error == nil {
				break
			}
		}
	}
	if result.Error != nil {
		return nil, fmt.Errorf("capability %s: %w", step.Capability, result.Error)
	}

	// Record successful capability output as knowledge.
	o.store.RecordKnowledge(wf.Name, "step_output",
		fmt.Sprintf("step=%s capability=%s output=%s",
			step.Name, step.Capability, truncate(result.Output, 150)),
		[]string{step.Name, step.Capability, wf.Name},
	)

	return &workflow.StepResult{
		StepName: step.Name,
		Output:   result.Output,
	}, nil
}

// execReason invokes the agent to reason about accumulated context.
func (o *Orchestrator) execReason(ctx context.Context, step *workflow.Step, ag *agent.Agent, stepContext string) (*workflow.StepResult, error) {
	output, err := ag.Reason(ctx, step.Name, step.Prompt, stepContext)
	if err != nil {
		return nil, err
	}
	return &workflow.StepResult{
		StepName: step.Name,
		Output:   output,
		Artifact: &workflow.Artifact{
			Name:     step.Name + "_reasoning",
			Kind:     "analysis",
			Content:  output,
			StepName: step.Name,
		},
	}, nil
}

// execTask spawns sub-tasks concurrently (multi-tasking).
// A task step contains its own sub-steps that run in a goroutine.
func (o *Orchestrator) execTask(ctx context.Context, wf *workflow.Workflow, step *workflow.Step, ag *agent.Agent, stepContext string) (*workflow.StepResult, error) {
	// For V1: task step executes as an independent named unit.
	// Multi-task concurrency: tasks declared together run concurrently.
	// Here a single task step delegates to the same orchestrator.
	o.log.Info("spawning task", "task", step.TaskName)

	var mu sync.Mutex
	var taskOutput string
	var taskErr error
	done := make(chan struct{})

	go func() {
		defer close(done)
		// For now, task step is a named reasoning unit.
		out, err := ag.Reason(ctx, step.TaskName, step.Prompt, stepContext)
		mu.Lock()
		taskOutput = out
		taskErr = err
		mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}

	mu.Lock()
	defer mu.Unlock()

	if taskErr != nil {
		return nil, fmt.Errorf("task %s: %w", step.TaskName, taskErr)
	}

	return &workflow.StepResult{
		StepName: step.Name,
		Output:   taskOutput,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
