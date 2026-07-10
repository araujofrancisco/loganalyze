package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/username/loganalyze/internal/server"
	"github.com/username/loganalyze/internal/summarizer"
)

var (
	flagAddr string
	flagData string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server with web UI",
	Run: func(cmd *cobra.Command, args []string) {
		if flagData == "" {
			flagData = os.TempDir()
		}
		if err := os.MkdirAll(flagData, 0755); err != nil {
			cmd.PrintErrln("cannot create data directory:", err)
			os.Exit(1)
		}

		var opts []server.Option
		endpoint := flagAIEndpoint
		if endpoint == "" {
			endpoint = os.Getenv("LOGANALYZE_AI_ENDPOINT")
		}
		model := flagAIModel
		if envModel := os.Getenv("LOGANALYZE_AI_MODEL"); envModel != "" {
			model = envModel
		}
		if endpoint != "" {
			opts = append(opts, server.WithSummarizer(
				summarizer.NewLLM(summarizer.Config{
					Endpoint: endpoint,
					Model:    model,
					APIKey:   os.Getenv("LOGANALYZE_AI_KEY"),
				}),
				model,
			))
		}

		srv := server.New(flagAddr, flagData, opts...)
		if err := srv.Start(); err != nil {
			cmd.PrintErrln("server error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&flagAddr, "addr", "a", ":8080", "listen address")
	serveCmd.Flags().StringVarP(&flagData, "data", "d", "/data", "data directory for uploads")
	serveCmd.Flags().StringVar(&flagAIEndpoint, "ai-endpoint", "", "OpenAI-compatible API endpoint for AI insights")
	serveCmd.Flags().StringVar(&flagAIModel, "ai-model", "gpt-4o-mini", "AI model name for insights")
	rootCmd.AddCommand(serveCmd)
}
