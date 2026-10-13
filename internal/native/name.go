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
// The browser's own file name wins, unchanged apart from characters Windows
// forbids. Media gets a name from the page title (see titleName) plus the
// extension of its type, since stream URLs end in names like
// "videoplayback"; other files leave the name to ODM's probe, because a page
// title rarely describes a linked archive or installer.
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
	name := titleName(title)
	if name == "" {
		return ""
	}
	return name + ext
}

// titleName turns a page title into a file name:
//   - the site name after the last " | " is dropped ("Episode 11 | Otaku
//     Desu" -> "Episode 11"); other separators such as " - " are kept, as
//     they are often part of the title itself;
//   - forbidden characters and spaces become "_", runs of "_" collapse, and
//     "_" or "." are trimmed from the ends;
//   - the result is capped at maxTitleName characters.
func titleName(title string) string {
	if i := strings.LastIndex(title, " | "); i > 0 {
		if head := strings.TrimSpace(title[:i]); head != "" {
			title = head
		}
	}
	name := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return '_'
		}
		return r
	}, clean(title))
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	name = strings.Trim(name, "_.")
	for utf8.RuneCountInString(name) > maxTitleName {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return strings.TrimRight(name, "_.")
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
