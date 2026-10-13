package clipmon

import (
	"context"
	"time"
)

// Config is read on every check, so turning the monitor on or off in
// Options, or editing the file types, applies without a restart.
type Config struct {
	Enabled   bool
	FileTypes []string
}

// Tracker decides which clipboard contents become downloads. It is fed the
// clipboard text whenever that may have changed, from whatever source the
// platform offers.
type Tracker struct {
	last    string
	started bool
	seen    map[string]bool
}

// Next returns the URL to queue for text, if any. The first text seen is
// only a baseline, and text that arrives while the monitor is off still
// moves the baseline: a link already on the clipboard when ODM starts, or
// when the monitor is switched on, is not grabbed; only later copies are.
// Each URL is queued at most once per run.
func (t *Tracker) Next(text string, cfg Config) (string, bool) {
	if t.seen == nil {
		t.seen = map[string]bool{}
	}
	changed := t.started && text != t.last
	t.started, t.last = true, text
	if !changed || !cfg.Enabled {
		return "", false
	}
	u, ok := ClipboardURL(text, cfg.FileTypes...)
	if !ok || t.seen[u] {
		return "", false
	}
	t.seen[u] = true
	return u, true
}

// Poll reads the clipboard with read every interval and hands new
// downloadable URLs to onURL until ctx ends. It suits platforms without a
// cheap change counter (the desktop UI reads through Fyne there).
func Poll(ctx context.Context, interval time.Duration, read func() string, cfg func() Config, onURL func(string)) {
	var t Tracker
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if u, ok := t.Next(read(), cfg()); ok {
			onURL(u)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
