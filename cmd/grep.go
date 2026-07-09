package cmd

import (
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/parser"
	"github.com/username/loganalyze/internal/reader"
	"github.com/username/loganalyze/internal/renderer"
)

var grepCmd = &cobra.Command{
	Use:   "grep [files...] <pattern>",
	Short: "Search log lines with a regex pattern",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pattern := args[len(args)-1]
		files := args[:len(args)-1]

		re, err := regexp.Compile(pattern)
		if err != nil {
			cmd.PrintErrln("invalid regex:", err)
			os.Exit(1)
		}

		cfg := buildFilterConfig()
		cfg.Regex = re

		lines := reader.ReadLines(files, len(files) == 0)

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
			renderer.PrintGrep(eventCh, os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(grepCmd)
}
