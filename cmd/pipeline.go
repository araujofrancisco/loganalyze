package cmd

import (
	"github.com/username/loganalyze/internal/filter"
	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/parser"
	"github.com/username/loganalyze/internal/reader"
)

func startPipeline(args []string, cfg filter.Config, limit int) chan model.Event {
	lines := reader.ReadLines(args, len(args) == 0)
	eventCh := make(chan model.Event, 1000)
	go func() {
		defer close(eventCh)
		count := 0
		for line := range lines {
			evt := parser.ParseLine(line.Text, line.Line, line.Source)
			if filter.Matches(evt, cfg) {
				eventCh <- evt
				count++
				if limit > 0 && count >= limit {
					break
				}
			}
		}
	}()
	return eventCh
}
