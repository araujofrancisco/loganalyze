package cmd

import (
	"os"

	"github.com/araujofrancisco/loganalyze/internal/model"
	"github.com/araujofrancisco/loganalyze/internal/renderer"
	"github.com/spf13/cobra"
)

var errorsCmd = &cobra.Command{
	Use:   "errors [files...]",
	Short: "Show error-level lines with context",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildFilterConfig()
		if cfg.MinLevel < model.LevelError {
			cfg.MinLevel = model.LevelError
		}
		eventCh := startPipeline(args, cfg, flagLimit)

		switch {
		case flagJSON:
			renderer.PrintEventsJSON(eventCh, os.Stdout)
		case flagCSV:
			renderer.PrintEventsCSV(eventCh, os.Stdout)
		default:
			renderer.PrintErrors(eventCh, os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(errorsCmd)
}
