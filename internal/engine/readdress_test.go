package engine

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAddAutoDuplicatePolicy(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	first, added := m.AddAuto(AddOptions{URL: "http://h/a.zip"})
	if !added {
		t.Fatal("first add refused")
	}
	again, added := m.AddAuto(AddOptions{URL: " http://h/a.zip#frag "})
	if added || again.ID != first.ID || len(m.List()) != 1 {
		t.Fatalf("duplicate added under skip policy: added=%v list=%d", added, len(m.List()))
	}
	st := m.Settings()
	st.DuplicateAuto = DuplicateAgain
	m.UpdateSettings(st)
	if _, added := m.AddAuto(AddOptions{URL: "http://h/a.zip"}); !added || len(m.List()) != 2 {
		t.Fatal("again policy did not add")
	}
}

// probedDownload registers a download that was probed against url but never
// transferred, as a link that expired before or during the first attempt.
func probedDownload(t *testing.T, m *Manager, url string, size int64) *Download {
	t.Helper()
	d := New(newID(), url, m.settings.Dir, 2, nil, 0, m.client)
	d.FinalURL, d.Total, d.RangeOK, d.FileName = url, size, true, "moved.bin"
	d.Chunks = NewPlan(size, 2)
	d.Status, d.Err = StatusError, "403 Forbidden"
	m.mu.Lock()
	m.downloads[d.ID] = d
	m.order = append(m.order, d.ID)
	m.mu.Unlock()
	return d
}

func TestReaddressContinuesFromNewURL(t *testing.T) {
	data := payload(256 * 1024)
	var hits int64
	fresh := rangeServer(t, data, &hits)
	defer fresh.Close()
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.client = fresh.Client()
	d := probedDownload(t, m, "http://expired.invalid/moved.bin", int64(len(data)))

	if err := m.Readdress(context.Background(), d.ID, fresh.URL+"/moved.bin"); err != nil {
		t.Fatal(err)
	}
	if info := d.Info(); info.Status != StatusPaused || info.Error != "" || !strings.HasPrefix(info.URL, fresh.URL) {
		t.Fatalf("after readdress: %+v", info)
	}
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete {
		t.Fatalf("status %s: %s", info.Status, info.Error)
	}
	if got, _ := os.ReadFile(filepath.Join(info.Dir, info.FileName)); !bytes.Equal(got, data) {
		t.Fatal("content from the new address does not match")
	}
}

func TestReaddressRejectsDifferentFile(t *testing.T) {
	var hits int64
	other := rangeServer(t, payload(1000), &hits)
	defer other.Close()
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.client = other.Client()
	d := probedDownload(t, m, "http://expired.invalid/x", 256*1024)

	err := m.Readdress(context.Background(), d.ID, other.URL+"/x")
	if err == nil || !strings.Contains(err.Error(), "different file") {
		t.Fatalf("size mismatch accepted: %v", err)
	}
	if d.Info().URL != "http://expired.invalid/x" {
		t.Fatal("address changed despite the error")
	}
}

func TestReaddressNeedsRangesOnceStarted(t *testing.T) {
	data := payload(256 * 1024)
	noRange := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer noRange.Close()
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.client = noRange.Client()
	d := probedDownload(t, m, "http://expired.invalid/y", int64(len(data)))
	d.Chunks[0].Done = 1000 // bytes already on disk

	if err := m.Readdress(context.Background(), d.ID, noRange.URL+"/y"); err == nil {
		t.Fatal("accepted a server that cannot resume")
	}
}

func TestReaddressRefusesWhileRunning(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	d := probedDownload(t, m, "http://expired.invalid/z", 10)
	d.Status = StatusDownloading
	if err := m.Readdress(context.Background(), d.ID, "http://new.example/z"); err == nil {
		t.Fatal("changed the address of a running download")
	}
	if err := m.Readdress(context.Background(), d.ID, "ftp://x/z"); err == nil {
		t.Fatal("accepted a non-http address")
	}
}
