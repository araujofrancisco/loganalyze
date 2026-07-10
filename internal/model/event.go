package model

import (
	"encoding/json"
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

func (l Level) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *Level) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, ok := ParseLevel(s)
	if !ok {
		*l = LevelInfo
		return nil
	}
	*l = parsed
	return nil
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
	Timestamp time.Time `json:"timestamp"`
	Level     Level     `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Raw       string    `json:"raw"`
	LineNum   int       `json:"line"`
}

type Group struct {
	Signature     string    `json:"signature"`
	SampleMessage string    `json:"sample"`
	Count         int       `json:"count"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	Index         int       `json:"-"`
}

type Report struct {
	Source     string         `json:"source"`
	TotalLines int            `json:"total_lines"`
	Levels     map[Level]int  `json:"-"`
	LevelsStr  map[string]int `json:"levels"`
	TopErrors  []Group        `json:"top_errors,omitempty"`
	FirstLine  time.Time      `json:"first_line"`
	LastLine   time.Time      `json:"last_line"`
}

func (r *Report) MarshalJSON() ([]byte, error) {
	type Alias Report
	r.LevelsStr = make(map[string]int, len(r.Levels))
	for lvl, count := range r.Levels {
		r.LevelsStr[lvl.String()] = count
	}
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(r)})
}

func (r *Report) UnmarshalJSON(data []byte) error {
	type Alias struct {
		Levels     map[string]int `json:"levels"`
		Source     string         `json:"source"`
		TotalLines int            `json:"total_lines"`
		TopErrors  []Group        `json:"top_errors"`
		FirstLine  time.Time      `json:"first_line"`
		LastLine   time.Time      `json:"last_line"`
	}
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	r.Source = a.Source
	r.TotalLines = a.TotalLines
	r.Levels = make(map[Level]int, len(a.Levels))
	for k, v := range a.Levels {
		lvl, _ := ParseLevel(k)
		r.Levels[lvl] = v
	}
	r.TopErrors = a.TopErrors
	r.FirstLine = a.FirstLine
	r.LastLine = a.LastLine
	return nil
}
