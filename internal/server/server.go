package server

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/session"
	"github.com/araujofrancisco/loganalyze/internal/summarizer"
	"github.com/araujofrancisco/loganalyze/internal/web"
)

type Server struct {
	addr       string
	dataDir    string
	sessions   *session.Store
	summarizer summarizer.Summarizer
	aiModel    string
	rateLimit  int
}

type Option func(*Server)

func WithSummarizer(s summarizer.Summarizer, model string) Option {
	return func(srv *Server) {
		srv.summarizer = s
		srv.aiModel = model
	}
}

func WithRateLimit(requestsPerMin int) Option {
	return func(srv *Server) {
		srv.rateLimit = requestsPerMin
	}
}

func New(addr, dataDir string, opts ...Option) *Server {
	srv := &Server{
		addr:      addr,
		dataDir:   dataDir,
		sessions:  session.NewStore(),
		rateLimit: 60,
	}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/results/{id}", s.handleResults)
	mux.HandleFunc("GET /api/results/{id}/events", s.handleEvents)
	mux.HandleFunc("GET /api/status/{id}", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
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

	handler := chain(mux, recoveryMiddleware, requestIDMiddleware, loggingMiddleware)

	if s.rateLimit > 0 {
		rl := newRateLimiter(s.rateLimit, 1*time.Minute)
		mux.Handle("POST /api/upload", rateLimitMiddleware(rl)(http.HandlerFunc(s.handleUpload)))
		mux.Handle("POST /api/analyze/{id}", rateLimitMiddleware(rl)(http.HandlerFunc(s.handleAnalyze)))
		mux.Handle("DELETE /api/sessions/{id}", rateLimitMiddleware(rl)(http.HandlerFunc(s.handleDeleteSession)))
	} else {
		mux.HandleFunc("POST /api/upload", s.handleUpload)
		mux.HandleFunc("POST /api/analyze/{id}", s.handleAnalyze)
		mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	}

	httpSrv := &http.Server{
		Addr:         s.addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 130 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sessions.Cleanup(1 * time.Hour)
			case <-ctx.Done():
				return
			}
		}
	}()

	if s.summarizer != nil {
		log.Printf("AI summarizer configured (model: %s, endpoint configured)", s.aiModel)
	}
	log.Printf("server listening on %s (data: %s)", s.addr, s.dataDir)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
		return err
	}

	log.Printf("server stopped")
	return nil
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

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + message + `"}`))
}
