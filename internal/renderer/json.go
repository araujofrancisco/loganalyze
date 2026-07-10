package renderer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/username/loganalyze/internal/model"
)

type jsonReport struct {
	Source     string         `json:"source"`
	TotalLines int            `json:"total_lines"`
	TimeRange  *jsonTimeRange `json:"time_range,omitempty"`
	Levels     map[string]int `json:"levels"`
	TopErrors  []jsonGroup    `json:"top_errors,omitempty"`
}

type jsonTimeRange struct {
	First       string `json:"first"`
	Last        string `json:"last"`
	DurationSec int64  `json:"duration_sec"`
}

type jsonGroup struct {
	Signature     string `json:"signature"`
	SampleMessage string `json:"sample"`
	Count         int    `json:"count"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
}

type jsonEvent struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Line      int    `json:"line"`
}

func PrintReportJSON(r model.Report, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	jr := jsonReport{
		Source:     r.Source,
		TotalLines: r.TotalLines,
		Levels:     make(map[string]int),
	}
	for lvl, count := range r.Levels {
		jr.Levels[lvl.String()] = count
	}
	if !r.FirstLine.IsZero() && !r.LastLine.IsZero() {
		jr.TimeRange = &jsonTimeRange{
			First:       r.FirstLine.Format(time.RFC3339Nano),
			Last:        r.LastLine.Format(time.RFC3339Nano),
			DurationSec: int64(r.LastLine.Sub(r.FirstLine).Seconds()),
		}
	}
	for _, g := range r.TopErrors {
		jr.TopErrors = append(jr.TopErrors, jsonGroup{
			Signature:     g.Signature,
			SampleMessage: g.SampleMessage,
			Count:         g.Count,
			FirstSeen:     g.FirstSeen.Format(time.RFC3339Nano),
			LastSeen:      g.LastSeen.Format(time.RFC3339Nano),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(jr)
}

func PrintEventJSON(evt model.Event) string {
	ts := ""
	if !evt.Timestamp.IsZero() {
		ts = evt.Timestamp.Format(time.RFC3339Nano)
	}
	je := jsonEvent{
		Timestamp: ts,
		Level:     evt.Level.String(),
		Source:    evt.Source,
		Message:   evt.Message,
		Line:      evt.LineNum,
	}
	b, _ := json.Marshal(je)
	return string(b)
}

func PrintEventsJSON(events <-chan model.Event, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	for evt := range events {
		fmt.Fprintln(w, PrintEventJSON(evt))
	}
}
