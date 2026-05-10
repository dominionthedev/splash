// Package dsl registers the Splash Lua API.
// This is the programming surface developers use to write workflows.
//
// Primitives:
//
//	workflow(name, { scope = scope_ref, steps = { ... } })
//	step(name, { ... })         -- wraps a step definition
//	execute(capability, params) -- run a capability
//	reason(prompt)              -- invoke constrained agent reasoning
//	task(name, prompt)          -- spawn a named sub-task
//	scope(name, { ... })        -- define an operational boundary
package dsl

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
	"github.com/dominionthedev/splash/internal/workflow"
)

// LoadResult is what the DSL returns after evaluating a Lua file.
type LoadResult struct {
	Workflows []*workflow.Workflow
	Scopes    map[string]*workflow.Scope
}

// Load evaluates a Lua workflow file and returns all defined workflows.
func Load(path string) (*LoadResult, error) {
	L := lua.NewState()
	defer L.Close()

	result := &LoadResult{
		Scopes: make(map[string]*workflow.Scope),
	}

	register(L, result)

	if err := L.DoFile(path); err != nil {
		return nil, fmt.Errorf("dsl: load %q: %w", path, err)
	}
	return result, nil
}

// LoadString evaluates Lua source and returns all defined workflows.
func LoadString(src string) (*LoadResult, error) {
	L := lua.NewState()
	defer L.Close()

	result := &LoadResult{
		Scopes: make(map[string]*workflow.Scope),
	}

	register(L, result)

	if err := L.DoString(src); err != nil {
		return nil, fmt.Errorf("dsl: parse: %w", err)
	}
	return result, nil
}

// register wires all DSL primitives into the Lua state.
func register(L *lua.LState, result *LoadResult) {
	// scope(name, { include = {}, exclude = {}, capabilities = {} })
	L.SetGlobal("scope", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		tbl := L.CheckTable(2)

		s := &workflow.Scope{Name: name}
		s.Include = stringList(tbl.RawGetString("include"))
		s.Exclude = stringList(tbl.RawGetString("exclude"))
		s.Capabilities = stringList(tbl.RawGetString("capabilities"))

		result.Scopes[name] = s

		// Return a userdata-like table so it can be assigned and referenced.
		ud := L.NewTable()
		ud.RawSetString("_scope_name", lua.LString(name))
		L.Push(ud)
		return 1
	}))

	// workflow(name, { scope = scope_ref, steps = { step(...), ... } })
	L.SetGlobal("workflow", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		tbl := L.CheckTable(2)

		wf := &workflow.Workflow{Name: name}

		// Resolve scope reference.
		if sv := tbl.RawGetString("scope"); sv != lua.LNil {
			if st, ok := sv.(*lua.LTable); ok {
				if sn := st.RawGetString("_scope_name"); sn != lua.LNil {
					scopeName := string(sn.(lua.LString))
					wf.Scope = result.Scopes[scopeName]
				}
			}
		}

		// Default empty scope if none declared.
		if wf.Scope == nil {
			wf.Scope = &workflow.Scope{Name: "default"}
		}

		// Parse steps.
		if sv := tbl.RawGetString("steps"); sv != lua.LNil {
			if st, ok := sv.(*lua.LTable); ok {
				st.ForEach(func(_, v lua.LValue) {
					if stepTbl, ok := v.(*lua.LTable); ok {
						if s := tableToStep(stepTbl); s != nil {
							wf.Steps = append(wf.Steps, s)
						}
					}
				})
			}
		}

		result.Workflows = append(result.Workflows, wf)
		return 0
	}))

	// step(name, body_table)
	// body_table contains exactly one of: execute(), reason(), task()
	L.SetGlobal("step", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		body := L.CheckTable(2)
		body.RawSetString("_step_name", lua.LString(name))
		L.Push(body)
		return 1
	}))

	// execute(capability_name, params_table?)
	L.SetGlobal("execute", L.NewFunction(func(L *lua.LState) int {
		capName := L.CheckString(1)
		tbl := L.NewTable()
		tbl.RawSetString("_kind", lua.LString("execute"))
		tbl.RawSetString("_capability", lua.LString(capName))
		if L.GetTop() >= 2 {
			if params, ok := L.Get(2).(*lua.LTable); ok {
				tbl.RawSetString("_params", params)
			}
		}
		L.Push(tbl)
		return 1
	}))

	// reason(prompt?)
	L.SetGlobal("reason", L.NewFunction(func(L *lua.LState) int {
		tbl := L.NewTable()
		tbl.RawSetString("_kind", lua.LString("reason"))
		if L.GetTop() >= 1 {
			if s, ok := L.Get(1).(lua.LString); ok {
				tbl.RawSetString("_prompt", s)
			}
		}
		L.Push(tbl)
		return 1
	}))

	// task(name, prompt?)
	L.SetGlobal("task", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		tbl := L.NewTable()
		tbl.RawSetString("_kind", lua.LString("task"))
		tbl.RawSetString("_task_name", lua.LString(name))
		if L.GetTop() >= 2 {
			if s, ok := L.Get(2).(lua.LString); ok {
				tbl.RawSetString("_prompt", s)
			}
		}
		L.Push(tbl)
		return 1
	}))
}

// tableToStep converts the result of a step(...) call into a *workflow.Step.
func tableToStep(tbl *lua.LTable) *workflow.Step {
	nameVal := tbl.RawGetString("_step_name")
	if nameVal == lua.LNil {
		return nil
	}
	stepName := string(nameVal.(lua.LString))

	kindVal := tbl.RawGetString("_kind")
	if kindVal == lua.LNil {
		return nil
	}
	kind := string(kindVal.(lua.LString))

	step := &workflow.Step{Name: stepName}

	switch kind {
	case "execute":
		step.Kind = workflow.StepExecute
		if cv := tbl.RawGetString("_capability"); cv != lua.LNil {
			step.Capability = string(cv.(lua.LString))
		}
		step.Params = extractParams(tbl.RawGetString("_params"))

	case "reason":
		step.Kind = workflow.StepReason
		if pv := tbl.RawGetString("_prompt"); pv != lua.LNil {
			step.Prompt = string(pv.(lua.LString))
		}

	case "task":
		step.Kind = workflow.StepTask
		if tn := tbl.RawGetString("_task_name"); tn != lua.LNil {
			step.TaskName = string(tn.(lua.LString))
		}
		if pv := tbl.RawGetString("_prompt"); pv != lua.LNil {
			step.Prompt = string(pv.(lua.LString))
		}
	}

	return step
}

// extractParams converts a Lua params table to map[string]string.
func extractParams(v lua.LValue) map[string]string {
	out := make(map[string]string)
	if v == lua.LNil {
		return out
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return out
	}
	tbl.ForEach(func(k, val lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			return
		}
		switch vs := val.(type) {
		case lua.LString:
			out[string(ks)] = string(vs)
		case lua.LNumber:
			out[string(ks)] = fmt.Sprintf("%g", float64(vs))
		case lua.LBool:
			out[string(ks)] = fmt.Sprintf("%v", bool(vs))
		}
	})
	return out
}

// stringList converts a Lua table of strings to []string.
func stringList(v lua.LValue) []string {
	if v == lua.LNil {
		return nil
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil
	}
	var out []string
	tbl.ForEach(func(_, val lua.LValue) {
		if s, ok := val.(lua.LString); ok {
			out = append(out, string(s))
		}
	})
	return out
}
