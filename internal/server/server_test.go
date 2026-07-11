package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/session"
)

func TestJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	jsonError(rec, http.StatusBadRequest, "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "bad request" {
		t.Errorf("error = %q, want bad request", body["error"])
	}
}

func TestJSONErrorWithSpecialChars(t *testing.T) {
	rec := httptest.NewRecorder()
	jsonError(rec, http.StatusInternalServerError, "internal error: something broke")

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body["error"], "internal") {
		t.Errorf("error = %q, should contain internal", body["error"])
	}
}

func TestServerHealth(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	handler := chain(http.HandlerFunc(srv.handleHealth),
		recoveryMiddleware,
		requestIDMiddleware,
	)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestServerGetSession(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/results/{id}", srv.handleResults)

	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	req := httptest.NewRequest("GET", "/api/results/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "session not found" {
		t.Errorf("error = %q, want session not found", body["error"])
	}
}

func TestServerUploadNoFile(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", srv.handleUpload)

	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	req := httptest.NewRequest("POST", "/api/upload", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServerAnalyzeInvalidCommand(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/analyze/{id}", srv.handleAnalyze)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	ses := srv.sessions.Create("test.log", "test.log", session.AnalyzeConfig{})

	body := strings.NewReader(`{"command":"invalid"}`)
	req := httptest.NewRequest("POST", "/api/analyze/"+ses.ID, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestServerAnalyzeValid(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/analyze/{id}", srv.handleAnalyze)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	ses := srv.sessions.Create("test.log", "test.log", session.AnalyzeConfig{})

	body := strings.NewReader(`{"command":"scan"}`)
	req := httptest.NewRequest("POST", "/api/analyze/"+ses.ID, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
}

func TestServerWithOptions(t *testing.T) {
	srv := New(":9999", "/tmp/data", WithRateLimit(5))
	if srv.rateLimit != 5 {
		t.Errorf("rateLimit = %d, want 5", srv.rateLimit)
	}
}

func TestServerHealthEndpointViaHTTPServer(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware, loggingMiddleware)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check: status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestServerSessionsEndpoint(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", srv.handleListSessions)
	mux.HandleFunc("POST /api/upload", srv.handleUpload)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	resp, err := http.Get(httptest.NewServer(handler).URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAnalyzeLimitClamping(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/analyze/{id}", srv.handleAnalyze)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware)

	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero limit", 0, 10},
		{"negative limit", -5, 10},
		{"small valid", 5, 5},
		{"max valid", 1000, 1000},
		{"over max", 5000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := srv.sessions.Create("test.log", "test.log", session.AnalyzeConfig{}).ID
			body := strings.NewReader(`{"command":"scan","limit":` + itoa(tt.input) + `}`)
			req := httptest.NewRequest("POST", "/api/analyze/"+id, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			ses := srv.sessions.Get(id)
			if ses.Config.Limit != tt.expected {
				t.Errorf("limit = %d, want %d", ses.Config.Limit, tt.expected)
			}
		})
	}
}

func TestServerWatchStreamsEvents(t *testing.T) {
	dir := t.TempDir()
	srv := New(":0", dir)

	path := dir + "/test.log"
	content := "ERROR: something broke\nINFO: all good\nERROR: another failure\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ses := srv.sessions.Create(path, "test.log", session.AnalyzeConfig{})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/watch/{id}", srv.handleWatch)
	handler := chain(mux, recoveryMiddleware, requestIDMiddleware, loggingMiddleware)

	// Connect as a real HTTP client
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Dial directly so we control the connection lifecycle
	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req := fmt.Sprintf("GET /api/watch/%s HTTP/1.1\r\nHost: %s\r\n\r\n", ses.ID, ts.Listener.Addr().String())
	conn.Write([]byte(req))

	// Read response — get all events, then context cancel will finish the handler
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	data, err := io.ReadAll(conn)
	if err != nil && !os.IsTimeout(err) {
		t.Fatal(err)
	}
	result := string(data)
	if !strings.Contains(result, "something broke") {
		t.Errorf("expected event content, got: %s", result)
	}
	if !strings.Contains(result, "type\":\"event") {
		t.Errorf("expected event type, got: %s", result)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
