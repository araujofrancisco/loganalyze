package summarizer

import "context"

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

type Summarizer interface {
	Summarize(ctx context.Context, req SummaryRequest) (*Summary, error)
	SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error)
}
