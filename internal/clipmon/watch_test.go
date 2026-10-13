package clipmon

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTrackerBaselineToggleAndDedupe(t *testing.T) {
	on, off := Config{Enabled: true}, Config{Enabled: false}
	var tr Tracker

	if _, ok := tr.Next("https://h/at-start.zip", on); ok {
		t.Fatal("grabbed what was on the clipboard at start")
	}
	if _, ok := tr.Next("https://h/while-off.zip", off); ok {
		t.Fatal("grabbed while the monitor was off")
	}
	// Switched on with the same text still there: not a new copy.
	if _, ok := tr.Next("https://h/while-off.zip", on); ok {
		t.Fatal("grabbed an old copy when the monitor was switched on")
	}
	if u, ok := tr.Next("see https://h/new.iso now", on); !ok || u != "https://h/new.iso" {
		t.Fatalf("new copy: %q %v", u, ok)
	}
	tr.Next("something else", on)
	if _, ok := tr.Next("https://h/new.iso", on); ok {
		t.Fatal("same URL queued twice")
	}
	if _, ok := tr.Next("https://h/page.html", on); ok {
		t.Fatal("non-download URL grabbed")
	}
	if u, ok := tr.Next("https://h/a.torrent", Config{Enabled: true, FileTypes: []string{"torrent"}}); !ok || u != "https://h/a.torrent" {
		t.Fatal("configured file type ignored")
	}
}

func TestPollQueuesLaterCopies(t *testing.T) {
	var mu sync.Mutex
	clip := "https://h/old.zip"
	var got []string
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Poll(ctx, 5*time.Millisecond,
			func() string { mu.Lock(); defer mu.Unlock(); return clip },
			func() Config { return Config{Enabled: true} },
			func(u string) { mu.Lock(); got = append(got, u); mu.Unlock() })
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	clip = "https://h/new.zip"
	mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if len(got) != 1 || got[0] != "https://h/new.zip" {
		t.Fatalf("queued %v, want only the later copy", got)
	}
}
