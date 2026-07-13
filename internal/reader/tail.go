package reader

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"time"
)

const pollInterval = 500 * time.Millisecond

func TailFile(ctx context.Context, path string, fromEnd bool) (<-chan Line, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var r io.ReadCloser = f
	if isGzip(f) {
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		r = gr
		// gzip is static — ignore fromEnd, seek doesn't apply
	} else if fromEnd {
		info, err := f.Stat()
		if err == nil {
			f.Seek(info.Size(), 0)
		}
	}

	ch := make(chan Line, 1000)
	go tailReader(ctx, r, path, ch)
	return ch, nil
}

func tailReader(ctx context.Context, r io.ReadCloser, source string, ch chan<- Line) {
	defer close(ch)
	defer r.Close()

	lineNum := 0
	var buf []byte
	readBuf := make([]byte, 32768)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := r.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			for {
				idx := -1
				for i, b := range buf {
					if b == '\n' {
						idx = i
						break
					}
				}
				if idx < 0 {
					break
				}
				lineNum++
				text := string(buf[:idx])
				if idx > 0 && buf[idx-1] == '\r' {
					text = text[:len(text)-1]
				}
				ch <- Line{Text: text, Source: source, Line: lineNum}
				buf = buf[idx+1:]
			}
		}

		if err != nil {
			if err == io.EOF {
				select {
				case <-ctx.Done():
					return
				case <-time.After(pollInterval):
				}
				continue
			}
			return
		}
	}
}
