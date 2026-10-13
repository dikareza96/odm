// Package update checks GitHub Releases for a newer ODM version. It only
// reports; it never downloads or replaces the binary. Self-replacement needs
// signed release assets first, otherwise a compromised release would push
// code to every install.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// DefaultAPI is the GitHub REST endpoint root.
const DefaultAPI = "https://api.github.com"

// Release is the part of a GitHub release the UI shows.
type Release struct {
	Tag   string `json:"tag_name"`
	URL   string `json:"html_url"`
	Notes string `json:"body"`
}

// Latest fetches the newest published release of repo ("owner/name").
// GitHub's /releases/latest skips drafts and pre-releases.
func Latest(ctx context.Context, client *http.Client, api, repo string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(api, "/")+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ODM-update-check") // GitHub rejects requests without one
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("no published release for %s", repo)
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release check: %s", resp.Status)
	}
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	if r.Tag == "" {
		return Release{}, fmt.Errorf("release check: response has no tag")
	}
	return r, nil
}

// Newer reports whether tag is a later version than current. Both may carry
// a leading "v" and a "-suffix"; suffixes are ignored, so a "0.1.0-debug"
// build is not offered the 0.1.0 release it was cut from. Unparseable input
// is never "newer", so a malformed tag cannot trigger a prompt.
func Newer(tag, current string) bool {
	a, okA := parse(tag)
	b, okB := parse(current)
	if !okA || !okB {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
