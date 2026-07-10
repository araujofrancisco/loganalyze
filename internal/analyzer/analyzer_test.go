package analyzer

import (
	"testing"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	ch := make(chan model.Event)
	close(ch)
	r := Analyze(ch, 10)
	if r.TotalLines != 0 {
		t.Errorf("total = %d, want 0", r.TotalLines)
	}
}

func TestAnalyzeCounts(t *testing.T) {
	ch := make(chan model.Event, 5)
	now := time.Now()
	for i := 0; i < 3; i++ {
		ch <- model.Event{Level: model.LevelError, Timestamp: now, Message: "error " + string(rune(i+'0'))}
	}
	ch <- model.Event{Level: model.LevelWarn, Timestamp: now, Message: "warning"}
	ch <- model.Event{Level: model.LevelInfo, Timestamp: now, Message: "info"}
	close(ch)

	r := Analyze(ch, 10)
	if r.TotalLines != 5 {
		t.Errorf("total = %d, want 5", r.TotalLines)
	}
	if r.Levels[model.LevelError] != 3 {
		t.Errorf("errors = %d, want 3", r.Levels[model.LevelError])
	}
	if r.Levels[model.LevelWarn] != 1 {
		t.Errorf("warns = %d, want 1", r.Levels[model.LevelWarn])
	}
}

func TestAnalyzeGrouping(t *testing.T) {
	ch := make(chan model.Event, 6)
	now := time.Now()
	for i := 0; i < 3; i++ {
		ch <- model.Event{
			Level:     model.LevelError,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Message:   "connection timeout after 30s on host 10.0.0.1",
		}
	}
	for i := 0; i < 2; i++ {
		ch <- model.Event{
			Level:     model.LevelError,
			Timestamp: now.Add(time.Duration(i+3) * time.Second),
			Message:   "connection timeout after 60s on host 10.0.0.2",
		}
	}
	close(ch)

	r := Analyze(ch, 10)
	if len(r.TopErrors) != 2 {
		t.Errorf("groups = %d, want 2", len(r.TopErrors))
	}
	top := r.TopErrors[0]
	if top.Count != 3 {
		t.Errorf("top group count = %d, want 3", top.Count)
	}
	if top.Signature != "connection timeout after 30s on host <ip>" {
		t.Errorf("signature = %q", top.Signature)
	}
}

func TestAnalyzeTopLimit(t *testing.T) {
	ch := make(chan model.Event, 10)
	now := time.Now()
	for i := 0; i < 5; i++ {
		ch <- model.Event{
			Level:     model.LevelError,
			Timestamp: now,
			Message:   "error type A",
		}
		ch <- model.Event{
			Level:     model.LevelError,
			Timestamp: now,
			Message:   "error type B",
		}
	}
	close(ch)

	r := Analyze(ch, 1)
	if len(r.TopErrors) > 1 {
		t.Errorf("limit=1 but got %d groups", len(r.TopErrors))
	}
}

func TestAnalyzeIgnoresNonErrors(t *testing.T) {
	ch := make(chan model.Event, 3)
	now := time.Now()
	ch <- model.Event{Level: model.LevelInfo, Timestamp: now, Message: "info msg"}
	ch <- model.Event{Level: model.LevelWarn, Timestamp: now, Message: "warn msg"}
	ch <- model.Event{Level: model.LevelError, Timestamp: now, Message: "error msg"}
	close(ch)

	r := Analyze(ch, 10)
	if len(r.TopErrors) != 1 {
		t.Errorf("groups = %d, want 1 (only errors grouped)", len(r.TopErrors))
	}
}

func TestAnalyzeFirstLastTime(t *testing.T) {
	ch := make(chan model.Event, 3)
	t1 := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 8, 14, 0, 0, 0, time.UTC)
	ch <- model.Event{Level: model.LevelInfo, Timestamp: t1, Message: "first"}
	ch <- model.Event{Level: model.LevelError, Timestamp: t2, Message: "middle"}
	ch <- model.Event{Level: model.LevelInfo, Timestamp: t3, Message: "last"}
	close(ch)

	r := Analyze(ch, 10)
	if !r.FirstLine.Equal(t1) {
		t.Errorf("first = %v, want %v", r.FirstLine, t1)
	}
	if !r.LastLine.Equal(t3) {
		t.Errorf("last = %v, want %v", r.LastLine, t3)
	}
}
