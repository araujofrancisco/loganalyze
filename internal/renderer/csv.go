package renderer

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
)

func PrintScanCSV(r model.Report, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	cw := csv.NewWriter(w)
	total := r.TotalLines
	if total == 0 {
		total = 1
	}
	cw.Write([]string{"level", "count", "pct"})
	for _, lvl := range []model.Level{model.LevelFatal, model.LevelError, model.LevelWarn, model.LevelInfo, model.LevelDebug} {
		count := r.Levels[lvl]
		pct := fmt.Sprintf("%.2f", float64(count)/float64(total)*100)
		cw.Write([]string{lvl.String(), fmt.Sprintf("%d", count), pct})
	}
	cw.Flush()
}

func PrintEventsCSV(events <-chan model.Event, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	cw := csv.NewWriter(w)
	cw.Write([]string{"timestamp", "level", "source", "message"})
	for evt := range events {
		ts := ""
		if !evt.Timestamp.IsZero() {
			ts = evt.Timestamp.Format(time.RFC3339Nano)
		}
		cw.Write([]string{ts, evt.Level.String(), evt.Source, evt.Message})
	}
	cw.Flush()
}
