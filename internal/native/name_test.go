package native

import (
	"strings"
	"testing"
)

func TestSuggestName(t *testing.T) {
	cases := []struct{ filename, title, mime, want string }{
		{`C:\Users\u\Downloads\setup.exe`, "Some page", "application/octet-stream", "setup.exe"},
		{"/home/u/Downloads/a.zip", "", "", "a.zip"},
		{"", "Dead Mount Death Play Part 2 Episode 10 Subtitle Indonesia", "video/mp4",
			"Dead Mount Death Play Part 2 Episode 10 Subtitle Indonesia.mp4"},
		{"", "Song: Live / 2026?", "audio/mpeg; codecs=mp3", "Song_ Live _ 2026_.mp3"},
		{"", "Release notes", "application/zip", ""}, // title says nothing about the archive
		{"", "", "video/mp4", ""},
	}
	for _, c := range cases {
		if got := SuggestName(c.filename, c.title, c.mime); got != c.want {
			t.Errorf("SuggestName(%q, %q, %q) = %q, want %q", c.filename, c.title, c.mime, got, c.want)
		}
	}
	long := SuggestName("", strings.Repeat("é", 300), "video/webm")
	if !strings.HasSuffix(long, ".webm") || len([]rune(long)) != maxTitleName+len(".webm") {
		t.Fatalf("long title not capped on a rune boundary: %d runes", len([]rune(long)))
	}
}
