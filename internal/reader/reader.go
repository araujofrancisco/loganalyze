package reader

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

type Line struct {
	Text   string
	Source string
	Line   int
}

func ReadLines(paths []string, stdin bool) chan Line {
	ch := make(chan Line)
	go func() {
		defer close(ch)
		if stdin || len(paths) == 0 {
			readReader(os.Stdin, "stdin", ch)
			return
		}
		for _, pattern := range paths {
			matches, err := filepath.Glob(pattern)
			if err != nil || len(matches) == 0 {
				matches = []string{pattern}
			}
			for _, fpath := range matches {
				readFile(fpath, ch)
			}
		}
	}()
	return ch
}

func readFile(path string, ch chan<- Line) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if isGzip(f) {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return
		}
		defer gr.Close()
		readReader(gr, path, ch)
		return
	}

	if isBinary(f) {
		return
	}
	readReader(f, path, ch)
}

func isGzip(f *os.File) bool {
	buf := make([]byte, 2)
	n, _ := f.Read(buf)
	if n < 2 {
		f.Seek(0, 0)
		return false
	}
	f.Seek(0, 0)
	return buf[0] == 0x1f && buf[1] == 0x8b
}

func readReader(r io.Reader, source string, ch chan<- Line) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		ch <- Line{Text: scanner.Text(), Source: source, Line: lineNum}
	}
}

func isBinary(f *os.File) bool {
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	if n == 0 {
		f.Seek(0, 0)
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			f.Seek(0, 0)
			return true
		}
	}
	f.Seek(0, 0)
	return false
}
