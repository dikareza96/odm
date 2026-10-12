package clipmon

import "testing"

func TestClipboardURL(t *testing.T) {
	cases := []struct {
		text string
		want string
		ok   bool
	}{
		{"https://example.com/movie.mp4", "https://example.com/movie.mp4", true},
		{"get it at https://cdn.host/path/archive.tar.gz now", "https://cdn.host/path/archive.tar.gz", true},
		{"see https://example.com/page", "", false},
		{"no links here", "", false},
		{"https://host/file.zip.", "https://host/file.zip", true},
		{"ftp://host/file.zip", "", false},
	}
	for _, c := range cases {
		got, ok := ClipboardURL(c.text)
		if ok != c.ok || got != c.want {
			t.Errorf("ClipboardURL(%q) = (%q,%v), want (%q,%v)", c.text, got, ok, c.want, c.ok)
		}
	}
}
