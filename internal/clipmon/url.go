package clipmon

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"odm/internal/engine"
)

var urlRe = regexp.MustCompile(`https?://[^\s"'<>()\[\]]+`)

// ClipboardURL returns the first HTTP(S) URL in text whose path names a file
// type ODM recognises as downloadable, built in or listed in extra.
func ClipboardURL(text string, extra ...string) (string, bool) {
	for _, raw := range urlRe.FindAllString(text, -1) {
		raw = strings.TrimRight(raw, ".,;:)")
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		if !engine.KnownFileType(path.Base(u.Path), extra...) {
			continue
		}
		return raw, true
	}
	return "", false
}
