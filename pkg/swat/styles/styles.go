package styles

import (
	"regexp"
	"strings"
)

func Clean(text string) string {
	clean := text
	r := regexp.MustCompile(`(?i)(\[[\\/]?[cub]\]|\[c[^\w][^\[\]]*?\])`)
	for {
		if !r.MatchString(clean) {
			break
		}
		clean = r.ReplaceAllString(clean, "")
	}
	return strings.TrimSpace(clean)
}

func ToHTML(text string) string {
	formatted := text
	// remove [b], [\b], [u], [\u] tags
	formatted = regexp.
		MustCompile(`(?i)\[(?:\\)?[bu]\]`).
		ReplaceAllString(formatted, "")
	// replace [c=XXXXXX] with spans
	formatted = regexp.
		MustCompile(`(?i)\[c[^\w]([a-f0-9]{6})\]([^\[]+)`).
		ReplaceAllString(formatted, `<span style="color:#$1;">$2</span>`)
	// remove unparsed and remaining [c=XXXXXX] and [\c] tags
	formatted = regexp.
		MustCompile(`(?i)\[(?:\\)?c(?:[^\w][^\[\]]*)?\]`).
		ReplaceAllString(formatted, "")
	return formatted
}
