package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/araujofrancisco/loganalyze/internal/model"
)

type TimestampInfo struct {
	Value  time.Time
	EndIdx int
}

var monthMap = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

func ParseLine(raw string, lineNum int, source string) model.Event {
	evt := model.Event{
		Raw:     raw,
		Source:  source,
		LineNum: lineNum,
	}

	ts := extractTimestamp(raw)
	evt.Timestamp = ts.Value
	remaining := raw[ts.EndIdx:]

	level, levelIdx := extractLevel(remaining)
	evt.Level = level

	msgStart := levelIdx
	if levelIdx >= 0 {
		msgStart = levelIdx + len(level.String())
	}

	msg := ""
	if msgStart > 0 && msgStart < len(remaining) {
		msg = strings.TrimSpace(remaining[msgStart:])
	} else if ts.EndIdx > 0 {
		msg = strings.TrimSpace(raw[ts.EndIdx:])
	} else {
		msg = strings.TrimSpace(raw)
	}
	msg = strings.TrimLeft(msg, ":-[]() \t")
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = strings.TrimSpace(raw)
	}
	evt.Message = msg

	return evt
}

func extractTimestamp(raw string) TimestampInfo {
	if m := reISO8601.FindStringIndex(raw); m != nil {
		ts := raw[m[0]:m[1]]
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", ts)
		}
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05Z07:00", ts)
		}
		if err == nil {
			return TimestampInfo{Value: t.UTC(), EndIdx: m[1]}
		}
	}

	if m := reDateSpace.FindStringIndex(raw); m != nil {
		ts := raw[m[0]:m[1]]
		t, err := time.ParseInLocation("2006-01-02 15:04:05", ts, time.UTC)
		if err != nil {
			t, err = time.ParseInLocation("2006-01-02 15:04:05.999999999", ts, time.UTC)
		}
		if err == nil {
			return TimestampInfo{Value: t.UTC(), EndIdx: m[1]}
		}
	}

	if m := reSyslog.FindStringSubmatchIndex(raw); m != nil {
		month := raw[m[2]:m[3]]
		day := raw[m[4]:m[5]]
		tim := raw[m[6]:m[7]]
		mon, ok := monthMap[month]
		if ok {
			now := time.Now().UTC()
			ts := fmt.Sprintf("%s-%s-%s %s", now.Format("2006"), padMonth(int(mon)), padDay(day), tim)
			t, err := time.ParseInLocation("2006-01-02 15:04:05", ts, time.UTC)
			if err == nil {
				if t.After(now.Add(24 * time.Hour)) {
					t = t.AddDate(-1, 0, 0)
				}
				return TimestampInfo{Value: t.UTC(), EndIdx: m[1]}
			}
		}
	}

	if m := reApache.FindStringIndex(raw); m != nil {
		ts := raw[m[0]:m[1]]
		t, err := time.Parse("02/Jan/2006:15:04:05 -0700", ts)
		if err == nil {
			return TimestampInfo{Value: t.UTC(), EndIdx: m[1]}
		}
	}

	if m := reEpoch.FindStringIndex(raw); m != nil {
		ts := raw[m[0]:m[1]]
		sec, err := strconv.ParseInt(ts, 10, 64)
		if err == nil {
			return TimestampInfo{Value: time.Unix(sec, 0).UTC(), EndIdx: m[1]}
		}
	}

	return TimestampInfo{}
}

func extractLevel(remaining string) (model.Level, int) {
	upper := strings.ToUpper(remaining)
	for _, keyword := range []string{"FATAL", "CRITICAL", "PANIC", "ERROR", "WARN", "WARNING", "INFO", "DEBUG", "TRACE"} {
		idx := strings.Index(upper, keyword)
		if idx < 0 {
			continue
		}
		before := idx == 0 || !isAlpha(upper[idx-1])
		after := idx+len(keyword) >= len(upper) || !isAlpha(upper[idx+len(keyword)])
		if before && after {
			l, _ := model.ParseLevel(keyword)
			return l, idx
		}
	}
	return model.LevelInfo, -1
}

func isAlpha(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func padMonth(m int) string {
	if m < 10 {
		return "0" + strconv.Itoa(m)
	}
	return strconv.Itoa(m)
}

func padDay(d string) string {
	d = strings.TrimSpace(d)
	if len(d) == 1 {
		return "0" + d
	}
	return d
}
