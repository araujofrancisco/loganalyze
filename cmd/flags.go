package cmd

import (
	"fmt"
	"regexp"
	"time"

	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/summarizer"
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

func buildSummaryRequest(r model.Report) summarizer.SummaryRequest {
	levels := make(map[string]int)
	for lvl, count := range r.Levels {
		levels[lvl.String()] = count
	}

	timeRange := ""
	if !r.FirstLine.IsZero() && !r.LastLine.IsZero() {
		timeRange = fmt.Sprintf("%s — %s",
			r.FirstLine.Format("2006-01-02 15:04:05"),
			r.LastLine.Format("2006-01-02 15:04:05"))
	}

	top := make([]summarizer.ErrorGroupSummary, len(r.TopErrors))
	for i, g := range r.TopErrors {
		top[i] = summarizer.ErrorGroupSummary{
			Signature:     g.Signature,
			SampleMessage: g.SampleMessage,
			Count:         g.Count,
		}
	}

	return summarizer.SummaryRequest{
		Source:     r.Source,
		TotalLines: r.TotalLines,
		Levels:     levels,
		TimeRange:  timeRange,
		TopErrors:  top,
	}
}
