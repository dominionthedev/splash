package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagModel     string
	flagWorkspace string
)

var rootCmd = &cobra.Command{
	Use:   "splash",
	Short: "A programmable operational runtime for development workflows",
	Long: `Splash is a workflow-native runtime.
You program how development work executes. The runtime executes it. The AI reasons inside it.

  splash run fix-tests               # run a workflow
  splash inspect fix-tests           # inspect a workflow definition`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagModel, "model", "llama3.2", "LLM model name")
	rootCmd.PersistentFlags().StringVar(&flagWorkspace, "workspace", "", "workspace root (default: cwd)")
}
