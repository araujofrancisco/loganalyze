package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/analyzer"
	"github.com/araujofrancisco/loganalyze/internal/filter"
	"github.com/araujofrancisco/loganalyze/internal/model"
	"github.com/araujofrancisco/loganalyze/internal/parser"
	"github.com/araujofrancisco/loganalyze/internal/reader"
	"github.com/araujofrancisco/loganalyze/internal/session"
	"github.com/araujofrancisco/loganalyze/internal/summarizer"
)

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "file too large (max 100MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := sanitizeFilename(header.Filename)
	if name == "" {
		name = fmt.Sprintf("upload_%d.log", time.Now().UnixNano())
	}

	b := make([]byte, 16)
	rand.Read(b)
	id := fmt.Sprintf("%x", b)
	dst := filepath.Join(s.dataDir, id+".log")

	f, err := os.Create(dst)
	if err != nil {
		log.Printf("error creating file: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		log.Printf("error saving file: %v", err)
		os.Remove(dst)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	ses := s.sessions.Create(dst, name, session.AnalyzeConfig{})
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": ses.ID})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	var req struct {
		Command string `json:"command"`
		Level   string `json:"level"`
		Regex   string `json:"regex"`
		Limit   int    `json:"limit"`
		Since   string `json:"since"`
		Until   string `json:"until"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Command {
	case "scan", "errors", "top", "grep":
	default:
		http.Error(w, "unknown command: "+req.Command, http.StatusBadRequest)
		return
	}

	ses.Config = session.AnalyzeConfig{
		Command: req.Command,
		Level:   req.Level,
		Regex:   req.Regex,
		Limit:   req.Limit,
		Since:   req.Since,
		Until:   req.Until,
	}
	ses.SetRunning()

	go s.runAnalysis(ses)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	resp := map[string]interface{}{
		"status":  ses.Status,
		"command": ses.Config.Command,
	}
	if ses.Status == "complete" {
		resp["report"] = ses.Report
		if ses.Events != nil {
			resp["events"] = ses.Events
		}
		if ses.Summary != nil {
			resp["summary"] = ses.Summary.Text
		}
	}
	if ses.Status == "error" {
		resp["error"] = ses.Error
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	if ses.Summary != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"summary": ses.Summary.Text,
			"model":   ses.Summary.ModelUsed,
			"cached":  true,
		})
		return
	}

	if s.summarizer == nil {
		http.Error(w, "AI summarizer not configured (set --ai-endpoint)", http.StatusNotImplemented)
		return
	}

	if ses.Status != "complete" {
		http.Error(w, "analysis not complete yet", http.StatusConflict)
		return
	}

	if ses.Report == nil {
		http.Error(w, "session has no report data", http.StatusBadRequest)
		return
	}

	req := summarizer.NewSummaryRequestFromReport(*ses.Report)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	summary, err := s.summarizer.Summarize(ctx, req)
	if err != nil {
		log.Printf("insights error for session %s: %v", ses.ID, err)
		http.Error(w, "AI summarization failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ses.SetSummary(summary)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary": summary.Text,
		"model":   summary.ModelUsed,
		"cached":  false,
	})
}

func (s *Server) handleInsightsStream(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	if s.summarizer == nil {
		http.Error(w, "AI summarizer not configured (set --ai-endpoint)", http.StatusNotImplemented)
		return
	}

	if ses.Status != "complete" {
		http.Error(w, "analysis not complete yet", http.StatusConflict)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// If already cached, stream the cached text
	if ses.Summary != nil {
		fmt.Fprintf(w, "data: {\"type\":\"text\",\"content\":%q}\n\n", ses.Summary.Text)
		fmt.Fprintf(w, "event: complete\ndata: {\"type\":\"done\"}\n\n")
		flusher.Flush()
		return
	}

	if ses.Report == nil {
		fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"content\":\"session has no report data\"}\n\n")
		flusher.Flush()
		return
	}

	req := summarizer.NewSummaryRequestFromReport(*ses.Report)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ch, err := s.summarizer.SummarizeStream(ctx, req)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"content\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	var fullText strings.Builder
	for token := range ch {
		if strings.HasPrefix(token, "error:") {
			fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"content\":%q}\n\n", token)
			flusher.Flush()
			return
		}
		fullText.WriteString(token)
		fmt.Fprintf(w, "data: {\"type\":\"text\",\"content\":%q}\n\n", token)
		flusher.Flush()
	}

	// Cache summary even if client disconnected
	ses.SetSummary(&summarizer.Summary{
		Text:      fullText.String(),
		ModelUsed: s.aiModel,
	})

	select {
	case <-r.Context().Done():
		return
	default:
	}

	fmt.Fprintf(w, "event: complete\ndata: {\"type\":\"done\"}\n\n")
	flusher.Flush()
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	status := ses.GetStatus()
	progress := ses.GetProgress()
	if status == "complete" {
		fmt.Fprintf(w, "event: complete\ndata: {\"status\":\"complete\"}\n\n")
		flusher.Flush()
		return
	}
	if status == "error" {
		fmt.Fprintf(w, "event: error\ndata: {\"status\":\"error\"}\n\n")
		flusher.Flush()
		return
	}
	if progress != "" {
		data := fmt.Sprintf(`{"status":"running","progress":%q}`, progress)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	done := r.Context().Done()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			status := ses.GetStatus()
			progress := ses.GetProgress()

			switch status {
			case "complete":
				fmt.Fprintf(w, "event: complete\ndata: {\"status\":\"complete\"}\n\n")
				flusher.Flush()
				return
			case "error":
				fmt.Fprintf(w, "event: error\ndata: {\"status\":\"error\"}\n\n")
				flusher.Flush()
				return
			default:
				if progress != "" {
					data := fmt.Sprintf(`{"status":"running","progress":%q}`, progress)
					fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
					flusher.Flush()
				}
			}
		}
	}
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessions.List()
	type sessionSummary struct {
		ID        string `json:"id"`
		FileName  string `json:"file_name"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	summaries := make([]sessionSummary, 0, len(sessions))
	for _, ses := range sessions {
		summaries = append(summaries, sessionSummary{
			ID:        ses.ID,
			FileName:  ses.FileName,
			Status:    ses.Status,
			CreatedAt: ses.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": summaries})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}
	os.Remove(ses.FilePath)
	s.sessions.Delete(ses.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}

	if ses.Status != "complete" || ses.Events == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []model.Event{},
			"total":  0,
			"offset": 0,
			"limit":  0,
		})
		return
	}

	offset := 0
	limit := 100
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := parseInt(o); err == nil && v >= 0 {
			offset = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}

	total := len(ses.Events)
	if offset >= total {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"events": []model.Event{},
			"total":  total,
			"offset": offset,
			"limit":  limit,
		})
		return
	}

	end := offset + limit
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": ses.Events[offset:end],
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (s *Server) handleRawUpload(w http.ResponseWriter, r *http.Request) {
	ses := s.getSession(w, r)
	if ses == nil {
		return
	}
	data, err := os.ReadFile(ses.FilePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) *session.Session {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return nil
	}
	ses := s.sessions.Get(id)
	if ses == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return nil
	}
	return ses
}

func (s *Server) runAnalysis(ses *session.Session) {
	cfg := buildFilterConfig(ses.Config)

	if _, err := os.Stat(ses.FilePath); os.IsNotExist(err) {
		ses.SetError("uploaded file not found")
		return
	}

	lines := reader.ReadLines([]string{ses.FilePath}, false)

	eventCh := make(chan model.Event, 1000)
	go func() {
		defer close(eventCh)
		parsedCount := 0
		matchCount := 0
		for line := range lines {
			parsedCount++
			evt := parser.ParseLine(line.Text, line.Line, line.Source)
			if filter.Matches(evt, cfg) {
				matchCount++
				if parsedCount%1000 == 0 {
					ses.SetProgress(fmt.Sprintf("parsed %d lines, %d matches", parsedCount, matchCount))
				}
				eventCh <- evt
			}
		}
		ses.SetProgress(fmt.Sprintf("parsing complete: %d lines, %d matches", parsedCount, matchCount))
	}()

	switch ses.Config.Command {
	case "scan", "top":
		limit := ses.Config.Limit
		if limit <= 0 {
			limit = 10
		}
		r := analyzer.Analyze(eventCh, limit)
		r.Source = ses.FileName
		ses.SetProgress("analysis complete")
		ses.SetComplete(&r, nil)
	case "errors", "grep":
		var events []model.Event
		for evt := range eventCh {
			events = append(events, evt)
		}
		ses.SetProgress(fmt.Sprintf("matched %d events", len(events)))
		ses.SetComplete(nil, events)
	}

	if s.summarizer != nil && ses.Report != nil {
		go s.generateSummary(ses)
	}
}

func buildFilterConfig(cfg session.AnalyzeConfig) filter.Config {
	fc := filter.Config{MinLevel: model.LevelDebug}
	if cfg.Level != "" {
		if lvl, ok := model.ParseLevel(cfg.Level); ok {
			fc.MinLevel = lvl
		}
	}
	if cfg.Regex != "" {
		fc.Regex = regexp.MustCompile(cfg.Regex)
	}
	if cfg.Since != "" {
		if d, err := time.ParseDuration(cfg.Since); err == nil {
			fc.Since = time.Now().Add(-d)
		}
	}
	if cfg.Until != "" {
		if t, err := time.Parse(time.RFC3339, cfg.Until); err == nil {
			fc.Until = t
		}
	}
	if cfg.Command == "errors" || cfg.Command == "top" {
		if fc.MinLevel < model.LevelError {
			fc.MinLevel = model.LevelError
		}
	}
	return fc
}

func sanitizeFilename(name string) string {
	var result []rune
	for _, r := range name {
		if r == '/' || r == '\\' || r == ':' {
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
