package reader

import (
	"testing"
)

func TestReadLinesPlain(t *testing.T) {
	ch := ReadLines([]string{"../../testdata/samples/errors.log"}, false)
	var count int
	for range ch {
		count++
	}
	if count != 15 {
		t.Fatalf("expected 15 lines from errors.log, got %d", count)
	}
}

func TestReadLinesGzip(t *testing.T) {
	ch := ReadLines([]string{"../../testdata/samples/errors.log.gz"}, false)
	var count int
	var lines []Line
	for l := range ch {
		count++
		lines = append(lines, l)
	}
	if count != 15 {
		t.Fatalf("expected 15 lines from errors.log.gz, got %d", count)
	}
	if lines[0].Source != "../../testdata/samples/errors.log.gz" {
		t.Errorf("source should be the gzip path, got %q", lines[0].Source)
	}
}

func TestReadLinesGzipContentMatch(t *testing.T) {
	plainCh := ReadLines([]string{"../../testdata/samples/errors.log"}, false)
	gzipCh := ReadLines([]string{"../../testdata/samples/errors.log.gz"}, false)

	var plainLines []Line
	var gzipLines []Line
	for l := range plainCh {
		plainLines = append(plainLines, l)
	}
	for l := range gzipCh {
		gzipLines = append(gzipLines, l)
	}

	if len(plainLines) != len(gzipLines) {
		t.Fatalf("line count mismatch: plain=%d gzip=%d", len(plainLines), len(gzipLines))
	}
	for i := range plainLines {
		if plainLines[i].Text != gzipLines[i].Text {
			t.Errorf("line %d mismatch:\nplain: %q\ngzip:  %q", i+1, plainLines[i].Text, gzipLines[i].Text)
		}
	}
}
