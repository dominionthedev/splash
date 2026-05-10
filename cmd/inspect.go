package cmd

import (
	"fmt"
	"os"

	"github.com/dominionthedev/splash/internal/dsl"
	"github.com/dominionthedev/splash/internal/workflow"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <workflow-file>",
	Short: "Inspect workflow definitions without running them",
	Long: `Parse and display the structure of a workflow file.

  splash inspect fix-tests.lua`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := dsl.Load(args[0])
		if err != nil {
			return err
		}

		if len(result.Workflows) == 0 {
			fmt.Fprintln(os.Stderr, "no workflows found")
			return nil
		}

		for _, wf := range result.Workflows {
			printWorkflow(wf)
			fmt.Println()
		}
		return nil
	},
}

func printWorkflow(wf *workflow.Workflow) {
	fmt.Printf("workflow: %s\n", wf.Name)

	if wf.Scope != nil {
		fmt.Printf("  scope: %s\n", wf.Scope.Name)
		if len(wf.Scope.Capabilities) > 0 {
			fmt.Printf("    capabilities: %v\n", wf.Scope.Capabilities)
		}
		if len(wf.Scope.Include) > 0 {
			fmt.Printf("    include: %v\n", wf.Scope.Include)
		}
		if len(wf.Scope.Exclude) > 0 {
			fmt.Printf("    exclude: %v\n", wf.Scope.Exclude)
		}
	}

	fmt.Printf("  steps (%d):\n", len(wf.Steps))
	for i, step := range wf.Steps {
		switch step.Kind {
		case workflow.StepExecute:
			fmt.Printf("    %d. [execute] %s → %s", i+1, step.Name, step.Capability)
			if len(step.Params) > 0 {
				fmt.Printf(" %v", step.Params)
			}
			if step.Retry > 0 {
				fmt.Printf(" (retry: %d)", step.Retry)
			}
			fmt.Println()
		case workflow.StepReason:
			fmt.Printf("    %d. [reason]  %s", i+1, step.Name)
			if step.Prompt != "" {
				fmt.Printf(": %q", step.Prompt)
			}
			fmt.Println()
		case workflow.StepTask:
			fmt.Printf("    %d. [task]    %s → %s\n", i+1, step.Name, step.TaskName)
		}
	}
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
