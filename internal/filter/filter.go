package filter

import (
	"regexp"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
)

type Config struct {
	Regex    *regexp.Regexp
	MinLevel model.Level
	Since    time.Time
	Until    time.Time
}

func Matches(evt model.Event, cfg Config) bool {
	if cfg.MinLevel > model.LevelDebug && evt.Level < cfg.MinLevel {
		return false
	}
	if cfg.Regex != nil && !cfg.Regex.MatchString(evt.Raw) {
		return false
	}
	if !cfg.Since.IsZero() && !evt.Timestamp.IsZero() && evt.Timestamp.Before(cfg.Since) {
		return false
	}
	if !cfg.Until.IsZero() && !evt.Timestamp.IsZero() && evt.Timestamp.After(cfg.Until) {
		return false
	}
	return true
}
