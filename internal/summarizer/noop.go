package summarizer

import "context"

type noop struct{}

func NewNoop() Summarizer {
	return &noop{}
}

func (n *noop) Summarize(_ context.Context, _ SummaryRequest) (*Summary, error) {
	return &Summary{
		Text: "AI summarization is not configured. Set --ai-endpoint or LOGANALYZE_AI_ENDPOINT to enable.",
	}, nil
}

func (n *noop) SummarizeStream(_ context.Context, _ SummaryRequest) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "AI summarization is not configured. Set --ai-endpoint or LOGANALYZE_AI_ENDPOINT to enable."
	close(ch)
	return ch, nil
}
