package renderer

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/username/loganalyze/internal/model"
)

var (
	styleFatal = color.New(color.FgWhite, color.BgRed, color.Bold)
	styleError = color.New(color.FgRed, color.Bold)
	styleWarn  = color.New(color.FgYellow, color.Bold)
	styleInfo  = color.New(color.FgCyan)
	styleDebug = color.New(color.Faint)
	styleDim   = color.New(color.Faint)
)

func PrintReport(r model.Report, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "File:\t%s\n", r.Source)
	fmt.Fprintf(tw, "Total lines:\t%d\n", r.TotalLines)
	if !r.FirstLine.IsZero() && !r.LastLine.IsZero() {
		fmt.Fprintf(tw, "Time range:\t%s — %s (%s)\n",
			r.FirstLine.Format("2006-01-02 15:04:05"),
			r.LastLine.Format("2006-01-02 15:04:05"),
			r.LastLine.Sub(r.FirstLine).Round(time.Second))
	}
	fmt.Fprintln(tw)

	total := r.TotalLines
	if total == 0 {
		total = 1
	}
	fmt.Fprintln(tw, "Level\tCount\t%")
	fmt.Fprintln(tw, "─────\t─────\t────")
	printLevelRow(tw, styleError, "ERROR", r.Levels[model.LevelError], total)
	printLevelRow(tw, styleWarn, "WARN", r.Levels[model.LevelWarn], total)
	printLevelRow(tw, styleInfo, "INFO", r.Levels[model.LevelInfo], total)
	printLevelRow(tw, styleDebug, "DEBUG", r.Levels[model.LevelDebug], total)
	printLevelRow(tw, styleFatal, "FATAL", r.Levels[model.LevelFatal], total)
	tw.Flush()

	if len(r.TopErrors) > 0 {
		fmt.Fprintln(w)
		tw2 := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		styleError.Fprintln(tw2, "Top errors:")
		for i, g := range r.TopErrors {
			first := g.FirstSeen.Format("15:04:05")
			last := g.LastSeen.Format("15:04:05")
			fmt.Fprintf(tw2, " %d.\t%s\t%dx\t[%s - %s]\n",
				i+1, g.Signature, g.Count, first, last)
		}
		tw2.Flush()
	}
}

func printLevelRow(tw *tabwriter.Writer, c *color.Color, name string, count, total int) {
	if count == 0 {
		c = styleDim
	}
	pct := float64(count) / float64(total) * 100
	c.Fprintf(tw, "%s\t%d\t%.1f%%\n", name, count, pct)
}

func PrintErrors(events <-chan model.Event, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	for evt := range events {
		line := formatEvent(evt)
		fmt.Fprintln(w, line)
	}
}

func PrintTop(r model.Report, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	if len(r.TopErrors) == 0 {
		fmt.Fprintln(w, "No errors found.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, g := range r.TopErrors {
		first := g.FirstSeen.Format("15:04:05")
		last := g.LastSeen.Format("15:04:05")
		styleError.Fprintf(tw, "%d.", i+1)
		fmt.Fprintf(tw, "\t%s\t%dx\t[%s - %s]\n", g.Signature, g.Count, first, last)
	}
	tw.Flush()
}

func PrintGrep(events <-chan model.Event, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	for evt := range events {
		line := formatEvent(evt)
		fmt.Fprintln(w, line)
	}
}

func formatEvent(evt model.Event) string {
	ts := ""
	if !evt.Timestamp.IsZero() {
		ts = evt.Timestamp.Format("2006-01-02 15:04:05")
	}
	levelStr := evt.Level.String()
	var c *color.Color
	switch evt.Level {
	case model.LevelFatal:
		c = styleFatal
	case model.LevelError:
		c = styleError
	case model.LevelWarn:
		c = styleWarn
	case model.LevelInfo:
		c = styleInfo
	default:
		c = styleDebug
	}
	return fmt.Sprintf("%s %s %s:%d %s",
		ts,
		c.Sprint(levelStr),
		evt.Source,
		evt.LineNum,
		evt.Message,
	)
}
