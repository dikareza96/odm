package engine

import (
	"net/url"
	"testing"
)

func TestExtractHrefs(t *testing.T) {
	html := `<a href="a.zip">a</a> <a href='/b.mp4'>b</a> <A HREF="c.pdf">c</A> <a name="x">`
	got := extractHrefs(html)
	want := []string{"a.zip", "/b.mp4", "c.pdf"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNormalizeURLDropsFragment(t *testing.T) {
	u, _ := url.Parse("https://host/page?x=1#frag")
	if got := normalizeURL(u); got != "https://host/page?x=1" {
		t.Fatalf("got %q", got)
	}
}
