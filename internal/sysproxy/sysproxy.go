// Package sysproxy resolves the proxy the operating system is configured to
// use, for downloads that have no proxy set in ODM's own options.
//
// Go's http.ProxyFromEnvironment reads only HTTP_PROXY/HTTPS_PROXY/NO_PROXY.
// On Windows most users set the proxy in Internet Options instead, which Go
// ignores, so behind a corporate proxy every download would fail. This
// package falls back to that per-user setting. Automatic configuration (PAC
// scripts, WPAD) is not evaluated; only a manually entered proxy is used.
package sysproxy

import (
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

// Config is a manual proxy setting as Windows stores it.
type Config struct {
	Enabled  bool
	Server   string // "host:port" or "http=host:port;https=host:port;…"
	Override string // "host;*.corp.example;<local>"
}

// For returns the proxy URL for a request target under cfg, or nil for a
// direct connection.
func (cfg Config) For(target *url.URL) *url.URL {
	if !cfg.Enabled || strings.TrimSpace(cfg.Server) == "" || cfg.bypass(target.Hostname()) {
		return nil
	}
	server := pickServer(cfg.Server, target.Scheme)
	if server == "" {
		return nil
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return nil
	}
	return u
}

// pickServer chooses the entry for scheme from a ProxyServer string. A plain
// "host:port" serves every scheme; in the per-protocol form HTTPS falls back
// to the http= entry, as browsers do.
func pickServer(s, scheme string) string {
	if !strings.Contains(s, "=") {
		return strings.TrimSpace(s)
	}
	entries := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			entries[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	if v := entries[strings.ToLower(scheme)]; v != "" {
		return v
	}
	return entries["http"]
}

// bypass applies ProxyOverride: "<local>" matches names without a dot and
// loopback addresses; other entries are case-insensitive wildcard patterns.
func (cfg Config) bypass(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" || isLoopback(host) {
		return true
	}
	for _, p := range strings.Split(cfg.Override, ";") {
		p = strings.ToLower(strings.TrimSpace(p))
		switch {
		case p == "":
		case p == "<local>":
			if !strings.Contains(host, ".") {
				return true
			}
		default:
			if ok, _ := path.Match(p, host); ok {
				return true
			}
		}
	}
	return false
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Func returns a Transport.Proxy function: environment variables first (so
// HTTPS_PROXY still wins when set), then the OS setting. The OS setting is
// re-read at most once a minute so a change applies without a restart.
func Func() func(*http.Request) (*url.URL, error) {
	var (
		mu     sync.Mutex
		cached Config
		readAt time.Time
	)
	return func(req *http.Request) (*url.URL, error) {
		if u, err := http.ProxyFromEnvironment(req); u != nil || err != nil {
			return u, err
		}
		mu.Lock()
		if time.Since(readAt) > time.Minute {
			cached, readAt = readSystem(), time.Now()
		}
		cfg := cached
		mu.Unlock()
		return cfg.For(req.URL), nil
	}
}
