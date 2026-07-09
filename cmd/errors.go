package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/parser"
	"github.com/username/loganalyze/internal/reader"
	"github.com/username/loganalyze/internal/renderer"
)

var errorsCmd = &cobra.Command{
	Use:   "errors [files...]",
	Short: "Show error-level lines with context",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildFilterConfig()
		if cfg.MinLevel < model.LevelError {
			cfg.MinLevel = model.LevelError
		}
		lines := reader.ReadLines(args, len(args) == 0)

		eventCh := make(chan model.Event, 1000)
		go func() {
			defer close(eventCh)
			for line := range lines {
				evt := parser.ParseLine(line.Text, line.Line, line.Source)
				if filter.Matches(evt, cfg) {
					eventCh <- evt
				}
			}
		}()

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
