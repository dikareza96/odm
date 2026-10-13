package native

import (
	"strings"
	"unicode/utf8"
)

// mediaExt maps media MIME types to file extensions.
var mediaExt = map[string]string{
	"video/mp4":        ".mp4",
	"video/webm":       ".webm",
	"video/x-matroska": ".mkv",
	"video/quicktime":  ".mov",
	"video/x-flv":      ".flv",
	"video/mp2t":       ".ts",
	"audio/mpeg":       ".mp3",
	"audio/mp4":        ".m4a",
	"audio/aac":        ".aac",
	"audio/ogg":        ".ogg",
	"audio/webm":       ".weba",
	"audio/flac":       ".flac",
	"audio/wav":        ".wav",
}

// maxTitleName keeps names from page titles well inside path limits.
const maxTitleName = 120

// SuggestName picks a file name for a download handed over by the browser.
// The browser's own file name wins. Media gets the page title plus the
// extension of its type, since stream URLs end in names like
// "videoplayback"; other files leave the name to ODM's probe, because a page
// title rarely describes a linked archive or installer. Path separators and
// characters Windows forbids are replaced.
func SuggestName(filename, title, mime string) string {
	if base := baseName(filename); base != "" {
		return clean(base)
	}
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	ext, ok := mediaExt[mime]
	title = strings.TrimSpace(title)
	if !ok || title == "" {
		return ""
	}
	name := clean(title)
	for utf8.RuneCountInString(name) > maxTitleName {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return strings.TrimRight(name, " .") + ext
}

func baseName(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// clean replaces characters that are invalid in Windows file names.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
