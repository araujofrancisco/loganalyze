package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/analyzer"
	"github.com/araujofrancisco/loganalyze/internal/filter"
	"github.com/araujofrancisco/loganalyze/internal/fold"
	"github.com/araujofrancisco/loganalyze/internal/model"
	"github.com/araujofrancisco/loganalyze/internal/parser"
	"github.com/araujofrancisco/loganalyze/internal/reader"
	"github.com/araujofrancisco/loganalyze/internal/renderer"
	"github.com/spf13/cobra"
)

var (
	flagEvery  string
	flagNoTail bool
)

var watchCmd = &cobra.Command{
	Use:   "watch [files...]",
	Short: "Tail log files with live filtering and periodic summaries",
	Long: `Watch log files in real-time. By default outputs matching lines as they arrive.

  --every 30s  Switch to periodic summary mode (aggregated report every N).
  --no-tail    Start from the beginning of the file instead of the end.`,
	Run: func(cmd *cobra.Command, args []string) {
		every, err := time.ParseDuration(flagEvery)
		if err != nil || every <= 0 {
			every = 0
		}

		cfg := buildFilterConfig()

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		lines := startTailPipeline(ctx, args, cfg, !flagNoTail)

		if every > 0 {
			runPeriodicWatch(ctx, lines, every, args)
		} else {
			runLiveWatch(ctx, lines)
		}
	},
}

func startTailPipeline(ctx context.Context, args []string, cfg filter.Config, fromEnd bool) chan model.Event {
	if len(args) == 0 {
		args = []string{"-"}
	}

	lineCh := make(chan reader.Line, 1000)

	go func() {
		defer close(lineCh)
		for _, arg := range args {
			if arg == "-" {
				ch := reader.ReadLines(nil, true)
				for line := range ch {
					select {
					case lineCh <- line:
					case <-ctx.Done():
						return
					}
				}
				continue
			}
			ch, err := reader.TailFile(ctx, arg, fromEnd)
			if err != nil {
				fmt.Fprintln(os.Stderr, "watch: cannot open", arg, err)
				continue
			}
			for line := range ch {
				select {
				case lineCh <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	if flagFold {
		lineCh = fold.Fold(lineCh, 50)
	}

	eventCh := make(chan model.Event, 1000)
	go func() {
		defer close(eventCh)
		for line := range lineCh {
			evt := parser.ParseLine(line.Text, line.Line, line.Source)
			if filter.Matches(evt, cfg) {
				eventCh <- evt
			}
		}
	}()
	return eventCh
}

func runLiveWatch(ctx context.Context, eventCh chan model.Event) {
	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				return
			}
			fmt.Fprintln(os.Stdout, renderer.FormatEvent(evt))
		case <-ctx.Done():
			return
		}
	}
}

func runPeriodicWatch(ctx context.Context, eventCh chan model.Event, every time.Duration, args []string) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	var allEvents []model.Event

	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				printPeriodicReport(allEvents, args)
				return
			}
			allEvents = append(allEvents, evt)
		case <-ticker.C:
			printPeriodicReport(allEvents, args)
		case <-ctx.Done():
			printPeriodicReport(allEvents, args)
			return
		}
	}
}

func printPeriodicReport(events []model.Event, args []string) {
	if len(events) == 0 {
		return
	}

	eventCh := make(chan model.Event, len(events))
	for _, evt := range events {
		eventCh <- evt
	}
	close(eventCh)

	r := analyzer.Analyze(eventCh, flagLimit)
	if len(args) > 0 {
		r.Source = args[0]
		r.Sources = args
	} else {
		r.Source = "stdin"
		r.Sources = []string{"stdin"}
	}

	switch {
	case flagJSON:
		renderer.PrintReportJSON(r, os.Stdout)
	case flagCSV:
		renderer.PrintScanCSV(r, os.Stdout)
	default:
		renderer.PrintReport(r, os.Stdout)
	}
	fmt.Fprintln(os.Stdout, "---")
}

func init() {
	watchCmd.Flags().StringVar(&flagEvery, "every", "", "periodic summary interval (e.g. 30s, 5m); default: live-line mode")
	watchCmd.Flags().BoolVar(&flagNoTail, "no-tail", false, "start from beginning of file")
	rootCmd.AddCommand(watchCmd)
}
