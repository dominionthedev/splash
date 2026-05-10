// Package agent provides the constrained reasoning layer.
// The agent does NOT control execution. It reasons inside boundaries
// defined by the workflow — scope, capabilities, and step instructions.
// It never acts outside what the workflow declares.
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/dominionthedev/splash/internal/capability"
	"github.com/dominionthedev/splash/internal/model"
	"github.com/dominionthedev/splash/internal/storage"
	"github.com/dominionthedev/splash/internal/workflow"
)

// Agent is the constrained reasoning system.
// One agent instance is created per workflow run.
type Agent struct {
	model    model.Model
	caps     *capability.Registry
	store    *storage.Store
	wf       *workflow.Workflow
	history  []model.Turn
}

// New creates an agent locked to the given workflow.
func New(m model.Model, caps *capability.Registry, store *storage.Store, wf *workflow.Workflow) *Agent {
	return &Agent{
		model: m,
		caps:  caps,
		store: store,
		wf:    wf,
	}
}

// Reason executes a StepReason — the agent reasons about the current context.
// It only has access to the workflow's declared capabilities and scope.
// It cannot execute capabilities itself — it returns reasoning output only.
func (a *Agent) Reason(ctx context.Context, stepName, prompt string, stepContext string) (string, error) {
	system := a.buildSystemPrompt()

	input := stepContext
	if prompt != "" {
		input = fmt.Sprintf("%s\n\n%s", prompt, stepContext)
	}

	reply, err := a.model.Chat(ctx, system, a.history, input)
	if err != nil {
		return "", fmt.Errorf("agent: reason at step %q: %w", stepName, err)
	}

	// Append to history so subsequent reason() steps have context.
	a.history = append(a.history,
		model.Turn{Role: "user", Content: input},
		model.Turn{Role: "assistant", Content: reply},
	)

	// Record to knowledge store.
	a.store.RecordKnowledge(
		a.wf.Name,
		"step_output",
		fmt.Sprintf("step=%s: %s", stepName, truncate(reply, 200)),
		[]string{stepName, a.wf.Name},
	)

	return reply, nil
}

// buildSystemPrompt constructs the agent's system message.
// It explicitly lists what the agent is allowed to do and what it cannot.
func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are the reasoning layer of Splash, a programmable operational runtime.\n")
	sb.WriteString("You do NOT control execution. You provide reasoning and analysis.\n")
	sb.WriteString("You operate strictly inside the boundaries defined by the current workflow.\n\n")

	// Workflow identity.
	fmt.Fprintf(&sb, "Workflow: %s\n\n", a.wf.Name)

	// Declared capabilities — the agent knows these exist but cannot invoke them.
	if a.wf.Scope != nil && len(a.wf.Scope.Capabilities) > 0 {
		sb.WriteString("Capabilities available in this workflow (executed by the runtime, not you):\n")
		sb.WriteString(a.caps.Describe(a.wf.Scope.Capabilities))
		sb.WriteString("\n")
	}

	// Scope context.
	if a.wf.Scope != nil {
		sb.WriteString("Operational scope:\n")
		if len(a.wf.Scope.Include) > 0 {
			fmt.Fprintf(&sb, "  include: %s\n", strings.Join(a.wf.Scope.Include, ", "))
		}
		if len(a.wf.Scope.Exclude) > 0 {
			fmt.Fprintf(&sb, "  exclude: %s\n", strings.Join(a.wf.Scope.Exclude, ", "))
		}
		sb.WriteString("\n")
	}

	// Inject relevant knowledge from previous runs.
	knowledge := a.store.KnowledgeSummary(a.wf.Name, 5)
	if knowledge != "" {
		sb.WriteString(knowledge)
		sb.WriteString("\n")
	}

	sb.WriteString("Respond with structured reasoning. Be precise. Do not hallucinate capability calls.")
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
