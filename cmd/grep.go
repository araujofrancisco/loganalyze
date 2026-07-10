package cmd

import (
	"os"
	"regexp"

	"github.com/araujofrancisco/loganalyze/internal/renderer"
	"github.com/spf13/cobra"
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

		eventCh := startPipeline(files, cfg, flagLimit)

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
