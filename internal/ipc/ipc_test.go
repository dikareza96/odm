package ipc

import (
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListenIsSingleInstanceAndServesHTTP(t *testing.T) {
	path := SocketPath(t.TempDir())
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	if _, err := Listen(path); !errors.Is(err, ErrRunning) {
		t.Fatalf("second Listen: %v, want ErrRunning", err)
	}

	resp, err := Client(path, 2*time.Second).Get(BaseURL + "/api/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "/api/ping" {
		t.Fatalf("body %q", body)
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), SocketName)
	// A socket file nobody listens on, as a crash leaves behind.
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if ul, ok := ln.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	ln.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stale socket not left behind: %v", err)
	}

	ln2, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen over stale socket: %v", err)
	}
	ln2.Close()
}
