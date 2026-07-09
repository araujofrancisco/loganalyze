package normalizer

import "regexp"

var patterns = []struct {
	re          *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "<uuid>"},
	{regexp.MustCompile(`\b(?:req|request|trace|span)[-_=][a-zA-Z0-9]{8,}\b`), "<req>"},
	{regexp.MustCompile(`(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|\b::1\b`), "<ip>"},
	{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "<ip>"},
	{regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`), "<hex>"},
	{regexp.MustCompile(`(?:/[^\s"')\]}:,;]*)+(?:\:\d+)?`), "<path>"},
	{regexp.MustCompile(`\b[0-9a-fA-F]{40,}\b`), "<hash>"},
	{regexp.MustCompile(`\b\d+\b`), "<n>"},
}

func Normalize(s string) string {
	for _, p := range patterns {
		s = p.re.ReplaceAllString(s, p.replacement)
	}
	return s
}
