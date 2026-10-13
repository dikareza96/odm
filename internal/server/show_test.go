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
