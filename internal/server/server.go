package server

import (
	"log"
	"net/http"
	"time"

	"github.com/username/loganalyze/internal/session"
	"github.com/username/loganalyze/internal/web"
)

type Server struct {
	addr     string
	dataDir  string
	sessions *session.Store
}

func New(addr, dataDir string) *Server {
	return &Server{
		addr:     addr,
		dataDir:  dataDir,
		sessions: session.NewStore(),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("POST /api/analyze/{id}", s.handleAnalyze)
	mux.HandleFunc("GET /api/results/{id}", s.handleResults)
	mux.HandleFunc("GET /api/status/{id}", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("GET /health", s.handleHealth)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(web.StaticFS))))
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

	log.Printf("server listening on %s (data: %s)", s.addr, s.dataDir)
	return http.ListenAndServe(s.addr, mux)
}
