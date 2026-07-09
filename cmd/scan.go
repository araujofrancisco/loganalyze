package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/username/loganalyze/internal/analyzer"
	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/parser"
	"github.com/username/loganalyze/internal/reader"
	"github.com/username/loganalyze/internal/renderer"
)

var scanCmd = &cobra.Command{
	Use:   "scan [files...]",
	Short: "Full analysis report of log files",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildFilterConfig()
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

		report := analyzer.Analyze(eventCh, flagLimit)
		if len(args) > 0 {
			report.Source = args[0]
		} else {
			report.Source = "stdin"
		}

		switch {
		case flagJSON:
			renderer.PrintReportJSON(report, os.Stdout)
		case flagCSV:
			renderer.PrintScanCSV(report, os.Stdout)
		default:
			renderer.PrintReport(report, os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
