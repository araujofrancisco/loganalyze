package summarizer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	req := SummaryRequest{
		Source:     "app.log",
		TotalLines: 1000,
		Levels:     map[string]int{"ERROR": 50, "WARN": 100, "INFO": 850},
		TimeRange:  "2026-07-08 10:00 — 14:00",
		TopErrors: []ErrorGroupSummary{
			{Signature: "timeout connecting to <ip>", SampleMessage: "timeout connecting to 10.0.0.5", Count: 30},
			{Signature: "disk full at <path>", SampleMessage: "disk full at /dev/sda1", Count: 10},
		},
	}

	prompt := buildPrompt(req)

	if !strings.Contains(prompt, "app.log") {
		t.Error("prompt should contain source")
	}
	if !strings.Contains(prompt, "1000") {
		t.Error("prompt should contain total lines")
	}
	if !strings.Contains(prompt, "ERROR: 50") {
		t.Error("prompt should contain level distribution")
	}
	if !strings.Contains(prompt, "timeout connecting to <ip>") {
		t.Error("prompt should contain error signature")
	}
	if !strings.Contains(prompt, "30x") {
		t.Error("prompt should contain count")
	}
}

func TestLLMSummarize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "The root cause appears to be network timeouts."}},
			},
			Usage: &struct {
				TotalTokens int `json:"total_tokens"`
			}{TotalTokens: 42},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	l := NewLLM(Config{
		Endpoint: srv.URL + "/v1",
		Model:    "test-model",
	})

	summary, err := l.Summarize(context.Background(), SummaryRequest{
		Source:     "test.log",
		TotalLines: 100,
		Levels:     map[string]int{"ERROR": 10},
		TopErrors:  []ErrorGroupSummary{{Signature: "test error", Count: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary.Text, "network timeouts") {
		t.Errorf("unexpected summary: %s", summary.Text)
	}
	if summary.ModelUsed != "test-model" {
		t.Errorf("model = %s, want test-model", summary.ModelUsed)
	}
}

func TestLLMSummarizeAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key-123" {
			t.Errorf("Authorization = %q, want Bearer test-key-123", auth)
		}
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	l := NewLLM(Config{
		Endpoint: srv.URL,
		Model:    "m",
		APIKey:   "test-key-123",
	})
	_, err := l.Summarize(context.Background(), SummaryRequest{Source: "x"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLLMSummarizeHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	l := NewLLM(Config{Endpoint: srv.URL, Model: "m"})
	_, err := l.Summarize(context.Background(), SummaryRequest{Source: "x"})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestLLMSummarizeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// hang forever
		select {}
	}))
	defer srv.Close()

	// Use a short client to force timeout
	l := &llm{
		config: Config{Endpoint: srv.URL, Model: "m"},
		client: &http.Client{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	_, err := l.Summarize(ctx, SummaryRequest{Source: "x"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestNoop(t *testing.T) {
	n := NewNoop()
	s, err := n.Summarize(context.Background(), SummaryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Text == "" {
		t.Error("noop should return a message")
	}
}

func TestNoopStream(t *testing.T) {
	n := NewNoop()
	ctx := context.Background()
	ch, err := n.SummarizeStream(ctx, SummaryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for s := range ch {
		parts = append(parts, s)
	}
	if len(parts) == 0 || parts[0] == "" {
		t.Error("noop stream should yield a message")
	}
}

func TestLLMStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response does not support flushing")
		}

		chunks := []string{"Root", " cause", " identified", "."}
		for _, c := range chunks {
			data := `{"choices":[{"delta":{"content":"` + c + `"},"finish_reason":null}]}`
			w.Write([]byte("data: " + data + "\n\n"))
			flusher.Flush()
		}

		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	l := NewLLM(Config{Endpoint: srv.URL, Model: "m"})
	ch, err := l.SummarizeStream(context.Background(), SummaryRequest{Source: "x"})
	if err != nil {
		t.Fatal(err)
	}

	var result string
	for s := range ch {
		result += s
	}
	if result != "Root cause identified." {
		t.Errorf("stream result = %q, want %q", result, "Root cause identified.")
	}
}
