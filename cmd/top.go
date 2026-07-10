package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/analyzer"
	"github.com/araujofrancisco/loganalyze/internal/model"
	"github.com/araujofrancisco/loganalyze/internal/renderer"
	"github.com/araujofrancisco/loganalyze/internal/summarizer"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top [files...]",
	Short: "Show top N recurring error patterns",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildFilterConfig()
		if cfg.MinLevel < model.LevelError {
			cfg.MinLevel = model.LevelError
		}
		eventCh := startPipeline(args, cfg, 0)

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
			renderer.PrintTop(report, os.Stdout)
		}

		endpoint, aiModel := getAIConfig()
		if endpoint != "" {
			s := summarizer.NewLLM(summarizer.Config{
				Endpoint: endpoint,
				Model:    aiModel,
				APIKey:   os.Getenv("LOGANALYZE_AI_KEY"),
			})
			req := summarizer.NewSummaryRequestFromReport(report)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			summary, err := s.Summarize(ctx, req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "AI summary error:", err)
			} else {
				renderer.PrintAISummary(summary, os.Stdout)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}
