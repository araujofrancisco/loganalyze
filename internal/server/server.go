package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/username/loganalyze/internal/session"
	"github.com/username/loganalyze/internal/summarizer"
	"github.com/username/loganalyze/internal/web"
)

type Server struct {
	addr       string
	dataDir    string
	sessions   *session.Store
	summarizer summarizer.Summarizer
	aiModel    string
}

type Option func(*Server)

func WithSummarizer(s summarizer.Summarizer, model string) Option {
	return func(srv *Server) {
		srv.summarizer = s
		srv.aiModel = model
	}
}

func New(addr, dataDir string, opts ...Option) *Server {
	srv := &Server{
		addr:     addr,
		dataDir:  dataDir,
		sessions: session.NewStore(),
	}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("POST /api/analyze/{id}", s.handleAnalyze)
	mux.HandleFunc("GET /api/results/{id}", s.handleResults)
	mux.HandleFunc("GET /api/results/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /api/status/{id}", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("GET /api/uploaded/{id}", s.handleRawUpload)
	mux.HandleFunc("GET /api/insights/{id}", s.handleInsights)
	mux.HandleFunc("GET /api/insights/{id}/stream", s.handleInsightsStream)
	mux.HandleFunc("GET /health", s.handleHealth)

	mux.Handle("GET /static/", http.FileServer(http.FS(web.StaticFS)))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := web.StaticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.sessions.Cleanup(1 * time.Hour)
		}
	}()

	if s.summarizer != nil {
		log.Printf("AI summarizer configured (model: %s, endpoint configured)", s.aiModel)
	}
	log.Printf("server listening on %s (data: %s)", s.addr, s.dataDir)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) generateSummary(ses *session.Session) {
	if ses.Report == nil {
		return
	}
	req := summarizer.NewSummaryRequestFromReport(*ses.Report)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	summary, err := s.summarizer.Summarize(ctx, req)
	if err != nil {
		log.Printf("background AI summary error for session %s: %v", ses.ID, err)
		return
	}
	ses.SetSummary(summary)
	log.Printf("background AI summary complete for session %s (model: %s)", ses.ID, summary.ModelUsed)
}
