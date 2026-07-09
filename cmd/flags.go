package cmd

import (
	"regexp"
	"time"

	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
)

func buildFilterConfig() filter.Config {
	cfg := filter.Config{
		MinLevel: model.LevelDebug,
	}

	if flagLevel != "" {
		lvl, ok := model.ParseLevel(flagLevel)
		if ok {
			cfg.MinLevel = lvl
		}
	}

	if flagRegex != "" {
		cfg.Regex = regexp.MustCompile(flagRegex)
	}

	if flagSince != "" {
		d, err := time.ParseDuration(flagSince)
		if err == nil {
			cfg.Since = time.Now().Add(-d)
		}
	}

	if flagUntil != "" {
		t, err := time.Parse(time.RFC3339, flagUntil)
		if err == nil {
			cfg.Until = t
		}
	}

	return cfg
}
