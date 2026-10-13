package native

import (
	"strings"
	"testing"
)

func TestSuggestName(t *testing.T) {
	cases := []struct{ filename, title, mime, want string }{
		// The browser's file name is kept as it is.
		{`C:\Users\u\Downloads\setup.exe`, "Some page", "application/octet-stream", "setup.exe"},
		{"/home/u/Downloads/My Report v2.pdf", "", "", "My Report v2.pdf"},
		// Media named from the page title: site name dropped, spaces to "_".
		{"", "Dead Mount Death Play Part 2 Episode 11 Subtitle Indonesia | Otaku Desu", "video/mp4",
			"Dead_Mount_Death_Play_Part_2_Episode_11_Subtitle_Indonesia.mp4"},
		{"", "Nama Video Luar Biasa", "video/webm", "Nama_Video_Luar_Biasa.webm"},
		{"", "Song: Live / 2026?", "audio/mpeg; codecs=mp3", "Song_Live_2026.mp3"},
		{"", "A | B | Site", "video/mp4", "A_B.mp4"},                      // only the last " | " part is the site
		{"", "Part 1 - The Start", "video/mp4", "Part_1_-_The_Start.mp4"}, // " - " is part of the title
		{"", " | Only Site", "video/mp4", "Only_Site.mp4"},                // nothing before the pipe: keep the rest
		{"", "...Trailer...", "video/mp4", "Trailer.mp4"},
		// No usable name: ODM's probe decides.
		{"", "Release notes", "application/zip", ""},
		{"", "", "video/mp4", ""},
		{"", "???", "video/mp4", ""},
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
