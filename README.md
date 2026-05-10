# splash

> A programmable operational runtime for development workflows.

Splash is **workflow-native**. You program how development work executes.
The runtime executes it. The AI reasons inside it — locked to what the workflow defines.

It is not an AI assistant. It is not a chatbot. It is a runtime you control.

---

## How it works

```
Developer  →  writes a Lua workflow
Runtime    →  executes it step by step
Agent      →  reasons inside workflow boundaries only
```

The agent cannot act outside what the workflow declares.
Every capability the agent can reference must be in the workflow's scope.
Every step is defined by the developer — not decided by the AI.

---

## Installation

### Via Go

Requires Go 1.22+.

```bash
go install github.com/dominionthedev/splash@latest
```

### From source

```bash
git clone https://github.com/dominionthedev/splash
cd splash
go build -o bin/splash ./
```

### Verify

```bash
splash --help
```

---

## Quick start

Write a workflow file:

```lua
-- fix-tests.lua

local my_scope = scope("implementation", {
    include      = { "src/**", "tests/**" },
    exclude      = { ".env", "secrets/**" },
    capabilities = { "process.execute", "filesystem.read" },
})

workflow("fix-tests", {
    scope = my_scope,

    steps = {
        step("run-tests",  execute("process.execute", { cmd = "go test ./... 2>&1" })),
        step("read-code",  execute("filesystem.read", { path = "main.go" })),
        step("analyze",    reason("Identify failures and propose a precise fix.")),
    }
})
```

Run it:

```bash
splash run fix-tests.lua

# Inspect without running
splash inspect fix-tests.lua

# Run a specific workflow from a multi-workflow file
splash run workflows.lua fix-tests
```

---

## DSL reference

| Primitive | Description |
|---|---|
| `scope(name, {...})` | Define an operational boundary — visible files, allowed capabilities |
| `workflow(name, {...})` | Define a workflow with a scope and ordered steps |
| `step(name, body)` | A named execution unit — wraps execute, reason, or task |
| `execute(capability, params)` | Run a capability. The runtime executes it |
| `reason(prompt?)` | Invoke constrained agent reasoning |
| `task(name, prompt?)` | Spawn a named concurrent sub-task |

### Built-in capabilities

| Capability | Params |
|---|---|
| `filesystem.read` | `path` |
| `filesystem.write` | `path`, `content` |
| `process.execute` | `cmd` |

---

## The agent constraint model

When a `reason()` step runs, the agent only receives:

- The workflow name
- Capabilities declared in the scope (descriptions only — it cannot invoke them)
- Scope boundaries (include/exclude paths)
- Accumulated outputs from previous steps
- Relevant knowledge from past runs (self-optimizing)

The agent **cannot** invoke capabilities. It **cannot** act outside the declared scope.
It reasons and returns structured text. The orchestrator drives everything else.

---

## Multi-tasking

`task()` steps spawn concurrent named sub-tasks:

```lua
workflow("code-review", {
    scope = review_scope,
    steps = {
        step("read",       execute("filesystem.read", { path = "main.go" })),
        step("security",   task("security-review",   "Check for vulnerabilities.")),
        step("perf",       task("performance-review","Check for inefficiencies.")),
        step("summarize",  reason("Synthesize all findings by severity.")),
    }
})
```

---

## Storage

Everything persists under `.splash/` in your workspace root:

```
.splash/
  knowledge/    — scored entries, reinforced across runs
  artifacts/    — outputs produced by reason() steps
  history/      — execution records
```

The knowledge store self-optimizes: entries from successful runs gain score,
entries from failures are penalized. The agent receives the highest-scoring
knowledge on every run — the runtime gets smarter over time.

---

## Model config

Splash talks to any Ollama-compatible server, including [OllaCloud](https://github.com/dominionthedev/ollacloud):

```bash
export OLLAMA_HOST=http://localhost:11434
# or
export OLLACLOUD_HOST=http://localhost:11434

splash run fix-tests.lua --model llama3.2
```

---

## License

[MIT](./LICENSE) — built by [DominionDev](https://github.com/dominionthedev).
