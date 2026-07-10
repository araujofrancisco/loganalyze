package summarizer

import (
	"context"
	"fmt"

	"github.com/username/loganalyze/internal/model"
)

type Config struct {
	Endpoint string
	Model    string
	APIKey   string
}

type SummaryRequest struct {
	Source     string
	TotalLines int
	Levels     map[string]int
	TimeRange  string
	TopErrors  []ErrorGroupSummary
}

type ErrorGroupSummary struct {
	Signature     string
	SampleMessage string
	Count         int
}

type Summary struct {
	Text      string
	ModelUsed string
}

func NewSummaryRequestFromReport(r model.Report) SummaryRequest {
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

	top := make([]ErrorGroupSummary, len(r.TopErrors))
	for i, g := range r.TopErrors {
		top[i] = ErrorGroupSummary{
			Signature:     g.Signature,
			SampleMessage: g.SampleMessage,
			Count:         g.Count,
		}
	}

	return SummaryRequest{
		Source:     r.Source,
		TotalLines: r.TotalLines,
		Levels:     levels,
		TimeRange:  timeRange,
		TopErrors:  top,
	}
}

type Summarizer interface {
	Summarize(ctx context.Context, req SummaryRequest) (*Summary, error)
	SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error)
}
