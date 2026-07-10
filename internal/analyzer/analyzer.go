package analyzer

import (
	"container/heap"
	"sort"
	"time"

	"github.com/username/loganalyze/internal/model"
	"github.com/username/loganalyze/internal/normalizer"
)

type Report = model.Report
type Group = model.Group

func Analyze(events <-chan model.Event, limit int) Report {
	r := Report{
		Levels: make(map[model.Level]int),
	}
	groups := make(map[string]*Group)
	gh := &groupHeap{}

	minTime := time.Time{}
	maxTime := time.Time{}

	for evt := range events {
		r.TotalLines++
		r.Levels[evt.Level]++

		if !evt.Timestamp.IsZero() {
			if minTime.IsZero() || evt.Timestamp.Before(minTime) {
				minTime = evt.Timestamp
			}
			if maxTime.IsZero() || evt.Timestamp.After(maxTime) {
				maxTime = evt.Timestamp
			}
		}

		if evt.Level < model.LevelError {
			continue
		}

		sig := normalizer.Normalize(evt.Message)
		if g, ok := groups[sig]; ok {
			g.Count++
			if evt.Timestamp.After(g.LastSeen) {
				g.LastSeen = evt.Timestamp
			}
			heap.Fix(gh, g.Index)
		} else {
			g = &Group{
				Signature:     sig,
				SampleMessage: evt.Message,
				Count:         1,
				FirstSeen:     evt.Timestamp,
				LastSeen:      evt.Timestamp,
			}
			if gh.Len() < limit || limit == 0 {
				heap.Push(gh, g)
				groups[sig] = g
			} else if gh.Len() > 0 && g.Count > (*gh)[0].Count {
				removed := heap.Remove(gh, 0).(*Group)
				delete(groups, removed.Signature)
				heap.Push(gh, g)
				groups[sig] = g
			}
		}
	}

	r.FirstLine = minTime
	r.LastLine = maxTime

	top := make([]Group, gh.Len())
	for i, g := range *gh {
		top[i] = *g
	}
	sort.Slice(top, func(i, j int) bool {
		return top[i].Count > top[j].Count
	})
	r.TopErrors = top

	return r
}

type groupHeap []*Group

func (h groupHeap) Len() int           { return len(h) }
func (h groupHeap) Less(i, j int) bool { return h[i].Count < h[j].Count }
func (h groupHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].Index = i; h[j].Index = j }
func (h *groupHeap) Push(x any)        { n := len(*h); g := x.(*Group); g.Index = n; *h = append(*h, g) }
func (h *groupHeap) Pop() any {
	old := *h
	n := len(old)
	g := old[n-1]
	g.Index = -1
	*h = old[:n-1]
	return g
}

var _ heap.Interface = (*groupHeap)(nil)
