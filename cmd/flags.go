package cmd

import (
	"os"
	"regexp"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/filter"
	"github.com/araujofrancisco/loganalyze/internal/model"
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

func getAIConfig() (endpoint, aiModel string) {
	endpoint = flagAIEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("LOGANALYZE_AI_ENDPOINT")
	}
	aiModel = flagAIModel
	if envModel := os.Getenv("LOGANALYZE_AI_MODEL"); envModel != "" {
		aiModel = envModel
	}
	return
}
