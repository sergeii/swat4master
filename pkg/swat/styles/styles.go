package styles

import (
	"regexp"
	"strings"
)

func Clean(text string) string {
	r := regexp.MustCompile(`(?i)(\[[\\/]?[cub]\]|\[c[^\w][^\[\]]*?\])`)
	for {
		if !r.MatchString(text) {
			break
		}
		text = r.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

func ToHTML(text string) string {
	// remove [b], [\b], [u], [\u] tags
	text = regexp.
		MustCompile(`(?i)\[(?:\\)?[bu]\]`).
		ReplaceAllString(text, "")
	// replace [c=XXXXXX] with spans
	text = regexp.
		MustCompile(`(?i)\[c[^\w]([a-f0-9]{6})\]([^\[]+)`).
		ReplaceAllString(text, `<span style="color:#$1;">$2</span>`)
	// remove unparsed and remaining [c=XXXXXX] and [\c] tags
	text = regexp.
		MustCompile(`(?i)\[(?:\\)?c(?:[^\w][^\[\]]*)?\]`).
		ReplaceAllString(text, "")
	return text
}
