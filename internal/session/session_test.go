package session

import (
	"sync"
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	st := NewStore()
	s := st.Create("/tmp/test.log", "test.log", AnalyzeConfig{})

	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
	if s.Status != "uploaded" {
		t.Errorf("status = %s, want uploaded", s.Status)
	}
	if s.FilePath != "/tmp/test.log" {
		t.Errorf("filePath = %s, want /tmp/test.log", s.FilePath)
	}
	if s.LastAccess.IsZero() {
		t.Error("LastAccess should be set on creation")
	}
}

func TestGetReturnsNilForMissing(t *testing.T) {
	st := NewStore()
	s := st.Get("nonexistent")
	if s != nil {
		t.Error("should return nil for missing session")
	}
}

func TestGetUpdatesLastAccess(t *testing.T) {
	st := NewStore()
	s := st.Create("/tmp/test.log", "test.log", AnalyzeConfig{})
	originalAccess := s.LastAccess

	time.Sleep(time.Millisecond)

	s2 := st.Get(s.ID)
	if s2 == nil {
		t.Fatal("session should exist")
	}
	if !s2.LastAccess.After(originalAccess) {
		t.Error("LastAccess should be updated on Get")
	}
}

func TestDelete(t *testing.T) {
	st := NewStore()
	s := st.Create("/tmp/test.log", "test.log", AnalyzeConfig{})
	st.Delete(s.ID)

	if st.Get(s.ID) != nil {
		t.Error("session should be deleted")
	}
}

func TestList(t *testing.T) {
	st := NewStore()
	st.Create("/tmp/a.log", "a.log", AnalyzeConfig{})
	st.Create("/tmp/b.log", "b.log", AnalyzeConfig{})

	list := st.List()
	if len(list) != 2 {
		t.Errorf("list length = %d, want 2", len(list))
	}
}

func TestListEmpty(t *testing.T) {
	st := NewStore()
	list := st.List()
	if len(list) != 0 {
		t.Errorf("list length = %d, want 0", len(list))
	}
}

func TestSetProgress(t *testing.T) {
	s := &Session{}
	s.SetProgress("parsing 1000 lines")
	if s.Progress != "parsing 1000 lines" {
		t.Errorf("progress = %q, want parsing 1000 lines", s.Progress)
	}
}

func TestGetProgress(t *testing.T) {
	s := &Session{}
	s.Progress = "done"
	if p := s.GetProgress(); p != "done" {
		t.Errorf("progress = %q, want done", p)
	}
}

func TestSetRunning(t *testing.T) {
	s := &Session{Status: "uploaded"}
	s.SetRunning()
	if s.Status != "running" {
		t.Errorf("status = %s, want running", s.Status)
	}
}

func TestSetError(t *testing.T) {
	s := &Session{}
	s.SetError("something broke")
	if s.Status != "error" {
		t.Errorf("status = %s, want error", s.Status)
	}
	if s.Error != "something broke" {
		t.Errorf("error = %q, want something broke", s.Error)
	}
}

func TestGetStatus(t *testing.T) {
	s := &Session{Status: "complete"}
	if st := s.GetStatus(); st != "complete" {
		t.Errorf("status = %s, want complete", st)
	}
}

func TestCleanup(t *testing.T) {
	st := NewStore()
	st.Create("/tmp/old.log", "old.log", AnalyzeConfig{})

	// Set last access to be very old
	st.mu.Lock()
	for _, s := range st.sessions {
		s.LastAccess = time.Now().Add(-2 * time.Hour)
	}
	st.mu.Unlock()

	st.Cleanup(1 * time.Hour)

	if len(st.List()) != 0 {
		t.Error("all sessions should be cleaned up")
	}
}

func TestCleanupSkipsRecent(t *testing.T) {
	st := NewStore()
	st.Create("/tmp/recent.log", "recent.log", AnalyzeConfig{})

	st.Cleanup(1 * time.Hour)

	if len(st.List()) != 1 {
		t.Error("recent session should not be cleaned up")
	}
}

func TestConcurrentAccess(t *testing.T) {
	st := NewStore()
	s := st.Create("/tmp/concurrent.log", "concurrent.log", AnalyzeConfig{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st.Get(s.ID)
			st.List()
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s2 := st.Create("/tmp/other.log", "other.log", AnalyzeConfig{})
			st.Delete(s2.ID)
		}()
	}
	wg.Wait()

	if st.Get(s.ID) == nil {
		t.Error("original session should still exist")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
	if len(id1) != 32 {
		t.Errorf("id length = %d, want 32", len(id1))
	}
}

func TestMultipleSessions(t *testing.T) {
	st := NewStore()

	sessions := make([]*Session, 10)
	for i := range sessions {
		path := "/tmp/test_" + itoa(i) + ".log"
		sessions[i] = st.Create(path, "test_"+itoa(i)+".log", AnalyzeConfig{})
	}

	if len(st.List()) != 10 {
		t.Errorf("list length = %d, want 10", len(st.List()))
	}

	for _, s := range sessions {
		got := st.Get(s.ID)
		if got == nil {
			t.Errorf("session %s should exist", s.ID)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
