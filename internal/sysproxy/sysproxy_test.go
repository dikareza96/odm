package sysproxy

import (
	"net/url"
	"testing"
)

func TestFor(t *testing.T) {
	u := func(s string) *url.URL { p, _ := url.Parse(s); return p }
	cases := []struct {
		name string
		cfg  Config
		dest string
		want string // "" = direct
	}{
		{"disabled", Config{Enabled: false, Server: "p:8080"}, "https://a.example/f", ""},
		{"single server", Config{Enabled: true, Server: "proxy.corp:8080"}, "https://a.example/f", "http://proxy.corp:8080"},
		{"per scheme https", Config{Enabled: true, Server: "http=h:80;https=s:443"}, "https://a.example/f", "http://s:443"},
		{"per scheme http", Config{Enabled: true, Server: "http=h:80;https=s:443"}, "http://a.example/f", "http://h:80"},
		{"https falls back to http", Config{Enabled: true, Server: "http=h:80;ftp=f:21"}, "https://a.example/f", "http://h:80"},
		{"socks only", Config{Enabled: true, Server: "socks=s:1080"}, "https://a.example/f", ""},
		{"<local> bypass", Config{Enabled: true, Server: "p:1", Override: "<local>"}, "http://intranet/f", ""},
		{"<local> keeps dotted", Config{Enabled: true, Server: "p:1", Override: "<local>"}, "http://a.example/f", "http://p:1"},
		{"wildcard bypass", Config{Enabled: true, Server: "p:1", Override: "*.corp.example;10.*"}, "http://files.corp.example/f", ""},
		{"ip wildcard bypass", Config{Enabled: true, Server: "p:1", Override: "*.corp.example;10.*"}, "http://10.1.2.3/f", ""},
		{"loopback always direct", Config{Enabled: true, Server: "p:1"}, "http://127.0.0.1:8080/f", ""},
	}
	for _, c := range cases {
		got := c.cfg.For(u(c.dest))
		gotS := ""
		if got != nil {
			gotS = got.String()
		}
		if gotS != c.want {
			t.Errorf("%s: got %q, want %q", c.name, gotS, c.want)
		}
	}
}
