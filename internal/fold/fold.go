package fold

import (
	"strings"

	"github.com/araujofrancisco/loganalyze/internal/reader"
)

const foldBufSize = 1000

func Fold(lines <-chan reader.Line, maxLines int) chan reader.Line {
	ch := make(chan reader.Line, foldBufSize)
	go func() {
		defer close(ch)
		var pending *reader.Line
		continuationCount := 0

		for line := range lines {
			if isContinuation(line.Text) && pending != nil && continuationCount < maxLines {
				pending.Text += "\n" + line.Text
				continuationCount++
				continue
			}
			if pending != nil {
				ch <- *pending
			}
			cp := line
			pending = &cp
			continuationCount = 0
		}

		if pending != nil {
			ch <- *pending
		}
	}()
	return ch
}

var continuationPrefixes = []string{"\t", "        ", "   ", "  ", " "}

func isContinuation(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, p := range continuationPrefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}
