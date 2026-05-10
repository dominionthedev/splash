// Package workflow contains the core data types for splash.
// A Workflow is the central unit — it defines execution structure,
// scope, and what the agent is allowed to reason about.
package workflow

// ── Workflow ───────────────────────────────────────────────────────────────

// Workflow is a programmable operational structure.
// It defines steps, scope, and agent behaviour.
// The runtime executes it. The agent reasons inside it.
type Workflow struct {
	Name  string
	Scope *Scope  // operational boundary for this workflow
	Steps []*Step // ordered execution steps
}

// ── Step ───────────────────────────────────────────────────────────────────

// StepKind classifies what a step does.
type StepKind string

const (
	// StepExecute runs a named capability.
	StepExecute StepKind = "execute"
	// StepReason invokes the agent to reason about the current context.
	// The agent is locked to the workflow's scope and declared capabilities.
	StepReason StepKind = "reason"
	// StepTask spawns a named sub-task (multi-tasking).
	StepTask StepKind = "task"
)

// Step is a single unit of execution inside a workflow.
type Step struct {
	Name       string
	Kind       StepKind
	Capability string            // populated when Kind == StepExecute
	TaskName   string            // populated when Kind == StepTask
	Prompt     string            // optional reasoning instruction for StepReason
	Params     map[string]string // runtime params passed to the capability
	Retry      int               // max retries on failure (default 0)
}

// ── Scope ──────────────────────────────────────────────────────────────────

// Scope defines operational boundaries for a workflow.
// The agent and capabilities operate within this boundary.
type Scope struct {
	Name         string
	Include      []string // glob patterns — visible paths
	Exclude      []string // glob patterns — blocked paths
	Capabilities []string // capability names the workflow can use
}

// AllowsCapability returns true if the scope permits the named capability.
func (s *Scope) AllowsCapability(name string) bool {
	if s == nil {
		return false
	}
	for _, c := range s.Capabilities {
		if c == name {
			return true
		}
	}
	return false
}

// ── Artifact ───────────────────────────────────────────────────────────────

// Artifact is an operational output produced during a run.
type Artifact struct {
	Name     string
	Kind     string // "plan" | "analysis" | "patch" | "output" | "report"
	Content  string
	StepName string // step that produced it
}

// ── ExecutionResult ────────────────────────────────────────────────────────

// StepResult captures the outcome of one step.
type StepResult struct {
	StepName string
	Output   string
	Artifact *Artifact // non-nil if this step produced an artifact
	Error    error
}

// RunResult is the full outcome of a workflow execution.
type RunResult struct {
	WorkflowName string
	Steps        []*StepResult
	Artifacts    []*Artifact
	Error        error // first fatal error, if any
}
