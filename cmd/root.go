package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagJSON       bool
	flagCSV        bool
	flagNoColor    bool
	flagLevel      string
	flagSince      string
	flagUntil      string
	flagLimit      int
	flagRegex      string
	flagAIEndpoint string
	flagAIModel    string
	flagFold       bool
)

var rootCmd = &cobra.Command{
	Use:   "loganalyze",
	Short: "Log analyzer — what's failing, when, how often, and where",
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagJSON, "json", "j", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&flagCSV, "csv", "c", false, "output as CSV")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable color output")
	rootCmd.PersistentFlags().StringVar(&flagLevel, "level", "", "minimum level (debug, info, warn, error, fatal)")
	rootCmd.PersistentFlags().StringVar(&flagSince, "since", "", "relative time filter (e.g. 1h, 30m)")
	rootCmd.PersistentFlags().StringVar(&flagUntil, "until", "", "absolute end time (RFC 3339)")
	rootCmd.PersistentFlags().IntVarP(&flagLimit, "limit", "n", 10, "max results")
	rootCmd.PersistentFlags().StringVar(&flagRegex, "regex", "", "regex pattern filter")
	rootCmd.PersistentFlags().StringVar(&flagAIEndpoint, "ai-endpoint", "", "OpenAI-compatible API endpoint for AI summary (also: LOGANALYZE_AI_ENDPOINT)")
	rootCmd.PersistentFlags().StringVar(&flagAIModel, "ai-model", "gpt-4o-mini", "AI model name for summary (also: LOGANALYZE_AI_MODEL)")
	rootCmd.PersistentFlags().BoolVar(&flagFold, "fold", false, "fold stack trace continuation lines into their parent error")
}
