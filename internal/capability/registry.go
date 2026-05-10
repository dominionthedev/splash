// Package capability manages runtime operations.
// Capabilities are abstract — the workflow declares which ones are accessible.
// The scope enforces access. Nothing executes outside a declared capability.
package capability

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// Input is the key-value params passed to a capability.
type Input map[string]string

// Result is what a capability returns.
type Result struct {
	Output string
	Error  error
}

// Capability is a named runtime operation.
type Capability interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input Input) Result
}

// Registry holds all registered capabilities.
type Registry struct {
	mu   sync.RWMutex
	caps map[string]Capability
}

// New returns a Registry pre-loaded with all built-in capabilities.
func New() *Registry {
	r := &Registry{caps: make(map[string]Capability)}
	r.Register(&fsRead{})
	r.Register(&fsWrite{})
	r.Register(&processExec{})
	return r
}

// Register adds a capability.
func (r *Registry) Register(c Capability) {
	r.mu.Lock()
	r.caps[c.Name()] = c
	r.mu.Unlock()
}

// Get returns a capability by name. Returns nil if not found.
func (r *Registry) Get(name string) Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.caps[name]
}

// Execute runs a named capability with the given input.
// Returns an error result if the capability isn't registered.
func (r *Registry) Execute(ctx context.Context, name string, input Input) Result {
	c := r.Get(name)
	if c == nil {
		return Result{Error: fmt.Errorf("capability %q not registered", name)}
	}
	return c.Execute(ctx, input)
}

// Describe returns formatted descriptions for a list of capability names.
// Used to inject the allowed capability list into the agent's prompt.
func (r *Registry) Describe(names []string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := ""
	for _, name := range names {
		if c, ok := r.caps[name]; ok {
			out += fmt.Sprintf("- %s: %s\n", c.Name(), c.Description())
		}
	}
	return out
}

// ── Built-in capabilities ─────────────────────────────────────────────────

// filesystem.read
type fsRead struct{}

func (f *fsRead) Name() string        { return "filesystem.read" }
func (f *fsRead) Description() string { return "Read a file. params: path" }
func (f *fsRead) Execute(_ context.Context, input Input) Result {
	path := input["path"]
	if path == "" {
		return Result{Error: fmt.Errorf("filesystem.read: missing param 'path'")}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{Error: fmt.Errorf("filesystem.read: %w", err)}
	}
	return Result{Output: string(data)}
}

// filesystem.write
type fsWrite struct{}

func (f *fsWrite) Name() string        { return "filesystem.write" }
func (f *fsWrite) Description() string { return "Write content to a file. params: path, content" }
func (f *fsWrite) Execute(_ context.Context, input Input) Result {
	path, content := input["path"], input["content"]
	if path == "" {
		return Result{Error: fmt.Errorf("filesystem.write: missing param 'path'")}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Result{Error: fmt.Errorf("filesystem.write: %w", err)}
	}
	return Result{Output: fmt.Sprintf("wrote %d bytes → %s", len(content), path)}
}

// process.execute
type processExec struct{}

func (p *processExec) Name() string        { return "process.execute" }
func (p *processExec) Description() string { return "Run a shell command. params: cmd" }
func (p *processExec) Execute(ctx context.Context, input Input) Result {
	cmd := input["cmd"]
	if cmd == "" {
		return Result{Error: fmt.Errorf("process.execute: missing param 'cmd'")}
	}
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	return Result{Output: string(out), Error: err}
}
