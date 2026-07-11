package fold

import (
	"testing"

	"github.com/araujofrancisco/loganalyze/internal/reader"
)

func TestFoldMergesContinuationLines(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 50)

	go func() {
		in <- reader.Line{Text: "Error: something broke", Source: "test.log", Line: 1}
		in <- reader.Line{Text: "  at main.foo(test.go:42)", Source: "test.log", Line: 2}
		in <- reader.Line{Text: "  at main.bar(test.go:100)", Source: "test.log", Line: 3}
		in <- reader.Line{Text: "normal line", Source: "test.log", Line: 4}
		close(in)
	}()

	var result []reader.Line
	for l := range out {
		result = append(result, l)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}

	expected := "Error: something broke\n  at main.foo(test.go:42)\n  at main.bar(test.go:100)"
	if result[0].Text != expected {
		t.Errorf("folded line mismatch:\ngot:  %q\nwant: %q", result[0].Text, expected)
	}

	if result[1].Text != "normal line" {
		t.Errorf("expected normal line unchanged, got: %q", result[1].Text)
	}
}

func TestFoldNotStart(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 50)

	go func() {
		in <- reader.Line{Text: "  indented first line", Source: "test.log", Line: 1}
		close(in)
	}()

	result := <-out
	if result.Text != "  indented first line" {
		t.Errorf("first line should pass through even if indented: %q", result.Text)
	}
}

func TestFoldMaxLines(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 2)

	go func() {
		in <- reader.Line{Text: "error start", Source: "test.log", Line: 1}
		in <- reader.Line{Text: "  frame1", Source: "test.log", Line: 2}
		in <- reader.Line{Text: "  frame2", Source: "test.log", Line: 3}
		in <- reader.Line{Text: "  frame3", Source: "test.log", Line: 4}
		in <- reader.Line{Text: "  frame4", Source: "test.log", Line: 5}
		close(in)
	}()

	result := <-out
	lines := 0
	for _, c := range result.Text {
		if c == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("expected 2 continuation lines, got %d (maxLines=2)", lines)
	}
}

func TestFoldEmptyChannel(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 50)
	close(in)

	count := 0
	for range out {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 output from empty channel, got %d", count)
	}
}

func TestFoldTabPrefix(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 50)

	go func() {
		in <- reader.Line{Text: "panic: runtime error", Source: "test.log", Line: 1}
		in <- reader.Line{Text: "\tgoroutine 1 [running]:", Source: "test.log", Line: 2}
		close(in)
	}()

	result := <-out
	if result.Text != "panic: runtime error\n\tgoroutine 1 [running]:" {
		t.Errorf("tab-prefixed continuation not folded: %q", result.Text)
	}
}

func TestFoldPreservesLineNumSource(t *testing.T) {
	in := make(chan reader.Line)
	out := Fold(in, 50)

	go func() {
		in <- reader.Line{Text: "error", Source: "app.log", Line: 42}
		in <- reader.Line{Text: "  continuation", Source: "ignored.log", Line: 99}
		close(in)
	}()

	result := <-out
	if result.Source != "app.log" {
		t.Errorf("expected source 'app.log', got %q", result.Source)
	}
	if result.Line != 42 {
		t.Errorf("expected line 42, got %d", result.Line)
	}
}
