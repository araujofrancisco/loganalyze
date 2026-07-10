package parser

import (
	"testing"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
)

func TestParseISOTimestamp(t *testing.T) {
	evt := ParseLine("2026-07-08T10:12:33Z ERROR database timeout", 1, "test.log")
	if evt.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if evt.Timestamp.Year() != 2026 || evt.Timestamp.Month() != 7 || evt.Timestamp.Day() != 8 {
		t.Errorf("bad date: %v", evt.Timestamp)
	}
	if evt.Level != model.LevelError {
		t.Errorf("level = %v, want ERROR", evt.Level)
	}
	if evt.Message != "database timeout" {
		t.Errorf("message = %q, want %q", evt.Message, "database timeout")
	}
}

func TestParseDateSpace(t *testing.T) {
	evt := ParseLine("2026-07-08 10:12:33 WARN disk 85%", 1, "test.log")
	if evt.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if evt.Level != model.LevelWarn {
		t.Errorf("level = %v, want WARN", evt.Level)
	}
	if evt.Message != "disk 85%" {
		t.Errorf("message = %q, want %q", evt.Message, "disk 85%")
	}
}

func TestParseSyslog(t *testing.T) {
	evt := ParseLine("Jul  8 10:12:33 hostname sshd[1234]: Failed password", 1, "test.log")
	if evt.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp for syslog")
	}
	_ = evt
}

func TestParseApache(t *testing.T) {
	evt := ParseLine(`192.168.1.1 - - [08/Jul/2026:10:12:33 +0000] "GET /api HTTP/1.1" 200 1234`, 1, "test.log")
	if evt.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp for apache")
	}
	level := evt.Level
	if level != model.LevelInfo {
		t.Logf("apache line level = %v (expected INFO as default)", level)
	}
}

func TestParseEpoch(t *testing.T) {
	evt := ParseLine("1720421553 ERROR something broke", 1, "test.log")
	if evt.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp for epoch")
	}
}

func TestParseNoTimestamp(t *testing.T) {
	evt := ParseLine("ERROR this has no timestamp", 1, "test.log")
	if !evt.Timestamp.IsZero() {
		t.Error("expected zero timestamp")
	}
	if evt.Level != model.LevelError {
		t.Errorf("level = %v, want ERROR", evt.Level)
	}
	if evt.Message != "this has no timestamp" {
		t.Errorf("message = %q", evt.Message)
	}
}

func TestParseUnknownLevel(t *testing.T) {
	evt := ParseLine("just some random log line here", 1, "test.log")
	if evt.Level != model.LevelInfo {
		t.Errorf("level = %v, want INFO (default)", evt.Level)
	}
}

func TestParseLevelCaseInsensitive(t *testing.T) {
	evt := ParseLine("2026-07-08T10:12:33Z error case insensitive", 1, "test.log")
	if evt.Level != model.LevelError {
		t.Errorf("level = %v, want ERROR", evt.Level)
	}
	evt2 := ParseLine("2026-07-08T10:12:33Z wArN mixed case", 2, "test.log")
	if evt2.Level != model.LevelWarn {
		t.Errorf("level = %v, want WARN", evt2.Level)
	}
}

func TestMessageNoTimestampInMessage(t *testing.T) {
	evt := ParseLine("2026-07-08T10:12:33Z ERROR database timeout", 1, "test.log")
	if containsTimestamp(evt.Message) {
		t.Errorf("message should not contain timestamp: %q", evt.Message)
	}
}

func containsTimestamp(s string) bool {
	return reISO8601.MatchString(s) || reDateSpace.MatchString(s)
}

func TestLineNum(t *testing.T) {
	evt := ParseLine("test", 42, "test.log")
	if evt.LineNum != 42 {
		t.Errorf("lineNum = %d, want 42", evt.LineNum)
	}
}

func TestSource(t *testing.T) {
	evt := ParseLine("test", 1, "app.log")
	if evt.Source != "app.log" {
		t.Errorf("source = %q, want app.log", evt.Source)
	}
}

var _ = time.Time{}
