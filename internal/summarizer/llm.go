package summarizer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type llm struct {
	config Config
	client *http.Client
}

func NewLLM(cfg Config) Summarizer {
	return &llm{
		config: cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func buildPrompt(req SummaryRequest) string {
	var b strings.Builder

	b.WriteString("You are a log analysis assistant. Given the following log summary, identify root causes, impacted components, and recommend next steps. Be concise (2-4 paragraphs).\n\n")

	fmt.Fprintf(&b, "Log source: %s\n", req.Source)
	fmt.Fprintf(&b, "Total lines analyzed: %d\n", req.TotalLines)
	if req.TimeRange != "" {
		fmt.Fprintf(&b, "Time range: %s\n", req.TimeRange)
	}

	b.WriteString("\nLevel distribution:\n")
	for _, lvl := range []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG"} {
		if count, ok := req.Levels[lvl]; ok && count > 0 {
			fmt.Fprintf(&b, "  %s: %d\n", lvl, count)
		}
	}

	if len(req.TopErrors) > 0 {
		b.WriteString("\nTop error patterns:\n")
		for i, g := range req.TopErrors {
			fmt.Fprintf(&b, "%d. [%dx] %s\n", i+1, g.Count, g.Signature)
			if g.SampleMessage != "" {
				fmt.Fprintf(&b, "   Sample: %s\n", g.SampleMessage)
			}
		}
	}

	return b.String()
}

func (l *llm) Summarize(ctx context.Context, req SummaryRequest) (*Summary, error) {
	prompt := buildPrompt(req)

	body := chatRequest{
		Model: l.config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a log analysis assistant. Be concise and direct."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(l.config.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.config.APIKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	return &Summary{
		Text:      strings.TrimSpace(chatResp.Choices[0].Message.Content),
		ModelUsed: l.config.Model,
	}, nil
}

func (l *llm) SummarizeStream(ctx context.Context, req SummaryRequest) (<-chan string, error) {
	prompt := buildPrompt(req)

	body := chatRequest{
		Model: l.config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a log analysis assistant. Be concise and direct."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
		Stream:      true,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(l.config.Endpoint, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.config.APIKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}

	ch := make(chan string, 16)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			ch <- fmt.Sprintf("error: HTTP %d - %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					select {
					case ch <- choice.Delta.Content:
					case <-ctx.Done():
						return
					}
				}
				if choice.FinishReason != nil && *choice.FinishReason == "stop" {
					return
				}
			}
		}
	}()

	return ch, nil
}
