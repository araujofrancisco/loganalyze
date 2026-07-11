package session

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
	"github.com/araujofrancisco/loganalyze/internal/summarizer"
)

type AnalyzeConfig struct {
	Command string
	Level   string
	Regex   string
	Limit   int
	Since   string
	Until   string
	Fold    bool
}

type Session struct {
	ID         string
	FilePath   string
	FileName   string
	Status     string
	Progress   string
	Report     *model.Report
	Events     []model.Event
	Summary    *summarizer.Summary
	Error      string
	CreatedAt  time.Time
	LastAccess time.Time
	Config     AnalyzeConfig
	mu         sync.RWMutex
}

func (s *Session) SetProgress(p string) {
	s.mu.Lock()
	s.Progress = p
	s.mu.Unlock()
}

func (s *Session) GetProgress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Progress
}

func (s *Session) SetComplete(report *model.Report, events []model.Event) {
	s.mu.Lock()
	s.Status = "complete"
	s.Report = report
	s.Events = events
	s.mu.Unlock()
}

func (s *Session) SetRunning() {
	s.mu.Lock()
	s.Status = "running"
	s.mu.Unlock()
}

func (s *Session) SetError(err string) {
	s.mu.Lock()
	s.Status = "error"
	s.Error = err
	s.mu.Unlock()
}

func (s *Session) SetSummary(summary *summarizer.Summary) {
	s.mu.Lock()
	s.Summary = summary
	s.mu.Unlock()
}

func (s *Session) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

func (st *Store) Create(filePath, fileName string, cfg AnalyzeConfig) *Session {
	id := generateID()
	now := time.Now()
	s := &Session{
		ID:         id,
		FilePath:   filePath,
		FileName:   fileName,
		Status:     "uploaded",
		CreatedAt:  now,
		LastAccess: now,
		Config:     cfg,
	}
	st.mu.Lock()
	st.sessions[id] = s
	st.mu.Unlock()
	return s
}

func (st *Store) Get(id string) *Session {
	st.mu.RLock()
	s, ok := st.sessions[id]
	st.mu.RUnlock()
	if ok {
		st.mu.Lock()
		s.LastAccess = time.Now()
		st.mu.Unlock()
	}
	return s
}

func (st *Store) Delete(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sessions, id)
}

func (st *Store) List() []*Session {
	st.mu.RLock()
	defer st.mu.RUnlock()
	list := make([]*Session, 0, len(st.sessions))
	for _, s := range st.sessions {
		list = append(list, s)
	}
	return list
}

func (st *Store) Cleanup(maxAge time.Duration) {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now()
	for id, s := range st.sessions {
		if now.Sub(s.LastAccess) > maxAge {
			delete(st.sessions, id)
		}
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
