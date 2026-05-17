package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/dominionthedev/splash/internal/capability"
	"github.com/dominionthedev/splash/internal/dsl"
	"github.com/dominionthedev/splash/internal/model"
	"github.com/dominionthedev/splash/internal/orchestrator"
	"github.com/dominionthedev/splash/internal/storage"
	"github.com/dominionthedev/splash/internal/workflow"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <workflow-file> [workflow-name]",
	Short: "Load and execute a workflow",
	Long: `Load a Lua workflow file and execute the named workflow (or first one found).

  splash run fix-tests.lua
  splash run workflows.lua fix-tests`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		targetName := ""
		if len(args) >= 2 {
			targetName = args[1]
		}

		// ── Load DSL ─────────────────────────────────────────────────────
		result, err := dsl.Load(filePath)
		if err != nil {
			return fmt.Errorf("failed to load %s:\n  %w", filePath, err)
		}
		if len(result.Workflows) == 0 {
			return fmt.Errorf("no workflows found in %s", filePath)
		}

		// Select target workflow.
		wf := result.Workflows[0]
		if targetName != "" {
			found := false
			for _, w := range result.Workflows {
				if w.Name == targetName {
					wf = w
					found = true
					break
				}
			}
			if !found {
				available := workflowNames(result.Workflows)
				return fmt.Errorf("workflow %q not found in %s\n  available: %s",
					targetName, filePath, strings.Join(available, ", "))
			}
		}

		// ── Runtime setup ─────────────────────────────────────────────────
		workspace := flagWorkspace
		if workspace == "" {
			workspace, _ = os.Getwd()
		}

		store, err := storage.New(workspace)
		if err != nil {
			return fmt.Errorf("storage init failed: %w", err)
		}

		caps := capability.New()
		m := model.NewOllama(flagModel)
		logger := log.New(os.Stderr)
		orch := orchestrator.New(caps, m, store, logger)

		// ── Execute ───────────────────────────────────────────────────────
		fmt.Fprintf(os.Stderr, "running workflow: %s (%d steps)\n\n", wf.Name, len(wf.Steps))
		runResult := orch.Run(context.Background(), wf)

		// ── Print results ─────────────────────────────────────────────────
		for _, sr := range runResult.Steps {
			printStepResult(sr)
		}
		fmt.Println()

		if runResult.Error != nil {
			fmt.Fprintf(os.Stderr, "workflow failed: %v\n", runResult.Error)
			os.Exit(1)
		}

		if len(runResult.Artifacts) > 0 {
			fmt.Fprintf(os.Stderr, "%d artifact(s) saved to .splash/artifacts/\n", len(runResult.Artifacts))
		}

		fmt.Fprintf(os.Stderr, "done.\n")
		return nil
	},
}

func printStepResult(sr *workflow.StepResult) {
	if sr.Error != nil {
		fmt.Printf("  ✗  %s\n", sr.StepName)
		// Indent each line of the error for readability.
		for _, line := range strings.Split(sr.Error.Error(), "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("     %s\n", line)
			}
		}
		return
	}

	fmt.Printf("  ✓  %s\n", sr.StepName)
	if sr.Output != "" {
		lines := strings.Split(strings.TrimRight(sr.Output, "\n"), "\n")
		// Cap output preview to 8 lines — full output is in artifacts.
		preview := lines
		truncated := false
		if len(lines) > 8 {
			preview = lines[:8]
			truncated = true
		}
		for _, line := range preview {
			fmt.Printf("     %s\n", line)
		}
		if truncated {
			fmt.Printf("     … (%d more lines)\n", len(lines)-8)
		}
	}
}

func workflowNames(wfs []*workflow.Workflow) []string {
	names := make([]string, len(wfs))
	for i, w := range wfs {
		names[i] = w.Name
	}
	return names
}

func init() {
	rootCmd.AddCommand(runCmd)
}
