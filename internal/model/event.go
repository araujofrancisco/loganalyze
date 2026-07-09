package model

import (
	"strings"
	"time"
)

type Level int8

const (
	LevelDebug Level = -1
	LevelInfo  Level = 0
	LevelWarn  Level = 1
	LevelError Level = 2
	LevelFatal Level = 3
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

var levelAliases = map[string]Level{
	"FATAL":    LevelFatal,
	"CRITICAL": LevelFatal,
	"PANIC":    LevelFatal,
	"ERROR":    LevelError,
	"WARN":     LevelWarn,
	"WARNING":  LevelWarn,
	"INFO":     LevelInfo,
	"DEBUG":    LevelDebug,
	"TRACE":    LevelDebug,
}

var levelOrder = []string{"FATAL", "CRITICAL", "PANIC", "ERROR", "WARN", "WARNING", "INFO", "DEBUG", "TRACE"}

func ParseLevel(s string) (Level, bool) {
	upper := strings.ToUpper(s)
	l, ok := levelAliases[upper]
	return l, ok
}

func DetectLevel(raw string) (Level, bool) {
	upper := strings.ToUpper(raw)
	for _, keyword := range levelOrder {
		idx := strings.Index(upper, keyword)
		if idx < 0 {
			continue
		}
		before := idx == 0 || !isAlpha(upper[idx-1])
		after := idx+len(keyword) >= len(upper) || !isAlpha(upper[idx+len(keyword)])
		if before && after {
			l, _ := ParseLevel(keyword)
			return l, true
		}
	}
	return LevelInfo, false
}

func isAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

type Event struct {
	Timestamp time.Time
	Level     Level
	Source    string
	Message   string
	Raw       string
	LineNum   int
}

type Group struct {
	Signature     string
	SampleMessage string
	Count         int
	FirstSeen     time.Time
	LastSeen      time.Time
	Index         int
}

type Report struct {
	Source     string
	TotalLines int
	Levels     map[Level]int
	TopErrors  []Group
	FirstLine  time.Time
	LastLine   time.Time
}
