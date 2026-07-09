package model

import (
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		ok    bool
	}{
		{"ERROR", LevelError, true},
		{"error", LevelError, true},
		{"Error", LevelError, true},
		{"WARN", LevelWarn, true},
		{"WARNING", LevelWarn, true},
		{"INFO", LevelInfo, true},
		{"DEBUG", LevelDebug, true},
		{"TRACE", LevelDebug, true},
		{"FATAL", LevelFatal, true},
		{"CRITICAL", LevelFatal, true},
		{"PANIC", LevelFatal, true},
		{"UNKNOWN", LevelInfo, false},
		{"", LevelInfo, false},
	}
	for _, tc := range tests {
		got, ok := ParseLevel(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseLevel(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDetectLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		ok    bool
	}{
		{"[ERROR] connection failed", LevelError, true},
		{"ERROR: timeout", LevelError, true},
		{"2026-07-08T10:12:33Z ERROR database timeout", LevelError, true},
		{"WARNING: disk full", LevelWarn, true},
		{"nothing here", LevelInfo, false},
		{"FATAL: kernel panic", LevelFatal, true},
	}
	for _, tc := range tests {
		got, ok := DetectLevel(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Errorf("DetectLevel(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLevelString(t *testing.T) {
	if LevelError.String() != "ERROR" {
		t.Errorf("LevelError.String() = %q, want ERROR", LevelError.String())
	}
	if LevelDebug.String() != "DEBUG" {
		t.Errorf("LevelDebug.String() = %q, want DEBUG", LevelDebug.String())
	}
}
