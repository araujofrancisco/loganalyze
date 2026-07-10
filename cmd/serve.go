package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/username/loganalyze/internal/server"
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
		srv := server.New(flagAddr, flagData)
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
