package filter

import (
	"regexp"
	"testing"
	"time"

	"github.com/username/loganalyze/internal/model"
)

func TestFilterLevel(t *testing.T) {
	evt := model.Event{Level: model.LevelError}

	cfg := Config{MinLevel: model.LevelError}
	if !Matches(evt, cfg) {
		t.Error("ERROR should match MinLevel=ERROR")
	}

	cfg = Config{MinLevel: model.LevelFatal}
	if Matches(evt, cfg) {
		t.Error("ERROR should NOT match MinLevel=FATAL")
	}

	cfg = Config{MinLevel: model.LevelWarn}
	if !Matches(evt, cfg) {
		t.Error("ERROR should match MinLevel=WARN")
	}
}

func TestFilterRegex(t *testing.T) {
	evt := model.Event{Raw: "database connection timeout after 30s"}
	cfg := Config{Regex: regexp.MustCompile("timeout")}
	if !Matches(evt, cfg) {
		t.Error("should match regex 'timeout'")
	}

	cfg = Config{Regex: regexp.MustCompile("panic")}
	if Matches(evt, cfg) {
		t.Error("should NOT match regex 'panic'")
	}
}

func TestFilterTime(t *testing.T) {
	now := time.Now()
	evt := model.Event{Timestamp: now}

	since := now.Add(-time.Hour)
	until := now.Add(time.Hour)
	cfg := Config{Since: since, Until: until}
	if !Matches(evt, cfg) {
		t.Error("event in range should match")
	}

	cfg = Config{Since: now.Add(time.Hour)}
	if Matches(evt, cfg) {
		t.Error("event before since should NOT match")
	}

	cfg = Config{Until: now.Add(-time.Hour)}
	if Matches(evt, cfg) {
		t.Error("event after until should NOT match")
	}
}

func TestFilterNoTimestamp(t *testing.T) {
	evt := model.Event{}
	cfg := Config{Since: time.Now()}
	if !Matches(evt, cfg) {
		t.Error("event with zero timestamp should pass through time filter (can't determine age)")
	}

	cfg = Config{}
	if !Matches(evt, cfg) {
		t.Error("event with no filters should always match")
	}
}

func TestFilterAllConditions(t *testing.T) {
	now := time.Now()
	evt := model.Event{
		Level:     model.LevelError,
		Raw:       "ERROR: disk full at /dev/sda1",
		Timestamp: now,
	}
	cfg := Config{
		MinLevel: model.LevelError,
		Regex:    regexp.MustCompile("disk"),
		Since:    now.Add(-time.Hour),
		Until:    now.Add(time.Hour),
	}
	if !Matches(evt, cfg) {
		t.Error("event matching ALL filters should pass")
	}
}
