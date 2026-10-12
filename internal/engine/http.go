package engine

import "net/http"

// applyHeaders sets the default user agent plus any caller-supplied headers.
// Caller headers win over the default user agent.
func applyHeaders(req *http.Request, headers map[string]string) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DefaultUserAgent)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}
