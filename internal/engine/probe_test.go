package engine

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeNameWindowsDevices(t *testing.T) {
	cases := map[string]string{
		"CON":         "_CON",
		"nul.mp4":     "_nul.mp4",
		"Com1.tar.gz": "_Com1.tar.gz",
		"LPT9 .txt":   "_LPT9 .txt",
		"COM²":        "_COM²",
		"conin$.log":  "_conin$.log",
		"CONSOLE.mp4": "CONSOLE.mp4",
		"COM10.mp4":   "COM10.mp4",
		"my nul.mp4":  "my nul.mp4",
		"aux_video":   "aux_video",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameCharacters(t *testing.T) {
	cases := map[string]string{
		"a\x00b\tc\x7fd.mp4":  "a_b_c_d.mp4",
		"bad\xffbyte.mp4":     "bad_byte.mp4",
		`x\y/z:w<>"|?*.txt`:   "x_y_z_w______.txt",
		"  trailing dot. ":    "trailing dot",
		"Épisode 01 — 日本.mkv": "Épisode 01 — 日本.mkv",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameLength(t *testing.T) {
	long := strings.Repeat("日本語", 40) + ".mp4" // 360 bytes of title
	got := SanitizeName(long)
	if len(got) > maxNameBytes || !utf8.ValidString(got) || !strings.HasSuffix(got, ".mp4") {
		t.Fatalf("%d bytes, valid %v: %q", len(got), utf8.ValidString(got), got)
	}
	// A cut that lands on a dot or space must not leave one at the end
	// of the name (Windows drops it, so the file would not be found).
	s := SanitizeName(strings.Repeat("a", 194) + " .  b.mp4")
	if strings.Contains(s, " .mp4") || strings.Contains(s, "..mp4") {
		t.Fatalf("cut left a trailing dot or space: %q", s)
	}
}
