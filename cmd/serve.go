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
		endpoint, aiModel := getAIConfig()
		if endpoint != "" {
			opts = append(opts, server.WithSummarizer(
				summarizer.NewLLM(summarizer.Config{
					Endpoint: endpoint,
					Model:    aiModel,
					APIKey:   os.Getenv("LOGANALYZE_AI_KEY"),
				}),
				aiModel,
			))
		}

		srv := server.New(flagAddr, flagData, opts...)
		if endpoint != "" {
			cmd.Println("AI summarizer configured (model:", aiModel, ")")
		}
		if err := srv.Start(); err != nil {
			cmd.PrintErrln("server error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringVarP(&flagAddr, "addr", "a", ":8080", "listen address")
	serveCmd.Flags().StringVarP(&flagData, "data", "d", "/data", "data directory for uploads")
	rootCmd.AddCommand(serveCmd)
}
