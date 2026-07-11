package reader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailFileReadsFromBeginning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := TailFile(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}

	var lines []Line
	for l := range ch {
		lines = append(lines, l)
		if len(lines) >= 3 {
			break
		}
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Text != "line1" {
		t.Errorf("expected line1, got %q", lines[0].Text)
	}
	if lines[0].Line != 1 {
		t.Errorf("expected line 1, got %d", lines[0].Line)
	}
	if lines[0].Source != path {
		t.Errorf("expected source %q, got %q", path, lines[0].Source)
	}
}

func TestTailFileFromEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("old\nlines\n"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	ch, err := TailFile(ctx, path, true)
	if err != nil {
		t.Fatal(err)
	}

	// Append new data after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString("new\nline\n")
			f.Close()
		}
	}()

	var lines []Line
	for l := range ch {
		lines = append(lines, l)
		if len(lines) >= 2 {
			break
		}
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 new lines, got %d", len(lines))
	}
	if lines[0].Text != "new" {
		t.Errorf("expected 'new', got %q", lines[0].Text)
	}
}

func TestTailFileNonexistent(t *testing.T) {
	ctx := context.Background()
	_, err := TailFile(ctx, "/nonexistent/path.log", false)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestTailFileContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("line1\nline2\n"), 0644)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := TailFile(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}

	// Read available lines, then cancel
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		done <- struct{}{}
	}()

	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("channel not closed after context cancellation")
	}
}

func TestTailFileEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.log")
	os.WriteFile(path, []byte{}, 0644)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := TailFile(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		count := 0
		for range ch {
			count++
		}
		if count != 0 {
			t.Errorf("expected 0 lines from empty file, got %d", count)
		}
		done <- struct{}{}
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("channel not closed after context cancellation")
	}
}
