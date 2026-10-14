package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"odm/internal/engine"
)

func TestShowRoute(t *testing.T) {
	s := New(engine.NewManager(engine.Settings{Dir: t.TempDir()}, ""))
	h := s.Handler()
	post := func() int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/show", nil))
		return w.Code
	}
	if c := post(); c != http.StatusNotFound {
		t.Fatalf("headless show: %d, want 404", c)
	}
	shown := 0
	s.OnShow = func() { shown++ }
	if c := post(); c != http.StatusNoContent || shown != 1 {
		t.Fatalf("show: %d, called %d times", c, shown)
	}
}

func TestAddReportsDuplicates(t *testing.T) {
	s := New(engine.NewManager(engine.Settings{Dir: t.TempDir()}, ""))
	h := s.Handler()
	add := func() int {
		w := httptest.NewRecorder()
		body := strings.NewReader(`{"url":"http://h/f.zip","start":false}`)
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", body))
		return w.Code
	}
	if c := add(); c != http.StatusCreated {
		t.Fatalf("first add: %d", c)
	}
	if c := add(); c != http.StatusOK {
		t.Fatalf("duplicate add: %d, want 200 (already in the list)", c)
	}
}

func TestBrowserAddPrompts(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	s := New(mgr)
	h := s.Handler()
	var prompted []engine.AddOptions
	s.OnPrompt = func(o engine.AddOptions) bool { prompted = append(prompted, o); return true }
	post := func(body string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
		return w.Code
	}

	if c := post(`{"url":"http://h/v.mp4","file_name":"Ep 10.mp4","prompt":true,"headers":{"Cookie":"a=1"}}`); c != http.StatusAccepted {
		t.Fatalf("prompted add: %d, want 202", c)
	}
	if len(prompted) != 1 || prompted[0].FileName != "Ep 10.mp4" || prompted[0].Headers["Cookie"] != "a=1" || len(mgr.List()) != 0 {
		t.Fatalf("prompt got %+v, list %d", prompted, len(mgr.List()))
	}
	if c := post(`{"url":"http://h/api.zip","start":false}`); c != http.StatusCreated || len(prompted) != 1 {
		t.Fatalf("script add without prompt: %d, prompts %d", c, len(prompted))
	}

	st := mgr.Settings()
	st.NoStartDialog = true
	mgr.UpdateSettings(st)
	if c := post(`{"url":"http://h/direct.zip","prompt":true,"start":false}`); c != http.StatusCreated || len(prompted) != 1 {
		t.Fatalf("dialog off: %d, prompts %d", c, len(prompted))
	}
}

func TestBrowserAddStream(t *testing.T) {
	mgr := engine.NewManager(engine.Settings{Dir: t.TempDir()}, "")
	s := New(mgr)
	h := s.Handler()
	prompts := 0
	s.OnPrompt = func(engine.AddOptions) bool { prompts++; return true }
	post := func(body string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(body)))
		return w.Code
	}

	if c := post(`{"url":"http://h/a/master.m3u8","stream":"hls","prompt":true,"start":false,"file_name":"Ep 1"}`); c != http.StatusAccepted {
		t.Fatalf("stream add: %d, want 202 (the window picks the quality)", c)
	}
	if prompts != 1 || len(mgr.List()) != 0 {
		t.Fatalf("prompts %d, list %d", prompts, len(mgr.List()))
	}
	// Without the window (headless, or turned off) it is queued directly.
	s.OnPrompt = nil
	if c := post(`{"url":"http://h/a/master.m3u8","stream":"hls","prompt":true,"start":false,"file_name":"Ep 1"}`); c != http.StatusCreated {
		t.Fatalf("stream add without window: %d", c)
	}
	list := mgr.List()
	if len(list) != 1 || list[0].Stream != "hls" || list[0].FileName != "Ep 1" {
		t.Fatalf("list %+v", list)
	}
	if c := post(`{"url":"http://h/a/x.ism/Manifest","stream":"smooth"}`); c != http.StatusBadRequest {
		t.Fatalf("unsupported stream: %d, want 400", c)
	}
}
