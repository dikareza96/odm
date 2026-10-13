package engine

import (
	"net/http"
	"sync/atomic"
)

// userAgent is the configured User-Agent. Downloads read it from their own
// goroutines while Options may change it, hence the atomic.
var userAgent atomic.Pointer[string]

func setUserAgent(ua string) {
	if ua == "" {
		ua = DefaultUserAgent
	}
	userAgent.Store(&ua)
}

func currentUserAgent() string {
	if p := userAgent.Load(); p != nil {
		return *p
	}
	return DefaultUserAgent
}

// applyHeaders sets the user agent plus any caller-supplied headers.
// Caller headers win over the user agent.
func applyHeaders(req *http.Request, headers map[string]string) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", currentUserAgent())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}
