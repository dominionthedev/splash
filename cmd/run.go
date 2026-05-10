package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/dominionthedev/splash/internal/capability"
	"github.com/dominionthedev/splash/internal/dsl"
	"github.com/dominionthedev/splash/internal/model"
	"github.com/dominionthedev/splash/internal/orchestrator"
	"github.com/dominionthedev/splash/internal/storage"
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

		// ── Load DSL ────────────────────────────────────────────────────
		result, err := dsl.Load(filePath)
		if err != nil {
			return err
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
				return fmt.Errorf("workflow %q not found in %s", targetName, filePath)
			}
		}

		// ── Setup runtime ───────────────────────────────────────────────
		workspace := flagWorkspace
		if workspace == "" {
			workspace, _ = os.Getwd()
		}

		store, err := storage.New(workspace)
		if err != nil {
			return fmt.Errorf("storage: %w", err)
		}

		caps := capability.New()
		m := model.NewOllama(flagModel)
		logger := log.New(os.Stderr)
		orch := orchestrator.New(caps, m, store, logger)

		// ── Execute ─────────────────────────────────────────────────────
		runResult := orch.Run(context.Background(), wf)

		// Print results.
		fmt.Println()
		for _, sr := range runResult.Steps {
			if sr.Error != nil {
				fmt.Fprintf(os.Stderr, "  ✗ [%s] %v\n", sr.StepName, sr.Error)
				continue
			}
			fmt.Printf("  ✓ [%s]\n", sr.StepName)
			if sr.Output != "" {
				indented := "      " + replaceNewlines(sr.Output, "\n      ")
				fmt.Println(indented)
			}
		}
		fmt.Println()

		if runResult.Error != nil {
			return fmt.Errorf("workflow failed: %w", runResult.Error)
		}

		if len(runResult.Artifacts) > 0 {
			fmt.Printf("  artifacts saved: %d\n", len(runResult.Artifacts))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func replaceNewlines(s, with string) string {
	out := ""
	for _, c := range s {
		if c == '\n' {
			out += with
		} else {
			out += string(c)
		}
	}
	return out
}
