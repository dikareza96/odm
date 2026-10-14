package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"odm/internal/native"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// relayed runs relayDownload against a fake ODM and returns what it posted.
func relayed(t *testing.T, m native.Message) (map[string]any, native.Reply) {
	t.Helper()
	var got map[string]any
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"id":"x"}`))}, nil
	})}
	reply := relayDownload(client, m)
	return got, reply
}

func TestRelayStream(t *testing.T) {
	got, reply := relayed(t, native.Message{
		Type: "download", Kind: "stream", URL: "https://cdn.example/v/master.m3u8?sig=1",
		Mime: "application/vnd.apple.mpegurl", Title: "Episode 3 | Site", Cookies: "a=1",
	})
	if !reply.OK || got["stream"] != "hls" || got["file_name"] != "Episode_3" {
		t.Fatalf("reply %+v, posted %+v", reply, got)
	}
	if h, _ := got["headers"].(map[string]any); h["Cookie"] != "a=1" {
		t.Fatalf("headers %+v", got["headers"])
	}

	// The extension calls a .mpd with an unhelpful type a stream too.
	got, _ = relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/m.mpd", Mime: "text/plain"})
	if got["stream"] != "dash" {
		t.Fatalf("dash: %+v", got)
	}
	got, _ = relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/p.m3u8", Mime: "video/mp2t"})
	if got["stream"] != "hls" {
		t.Fatalf("mislabelled hls: %+v", got)
	}
}

func TestRelayStreamUnsupported(t *testing.T) {
	_, reply := relayed(t, native.Message{Kind: "stream", URL: "https://cdn.example/x.ism/Manifest"})
	if reply.OK || reply.Error == "" {
		t.Fatalf("smooth stream accepted: %+v", reply)
	}
}

func TestRelayFileHasNoStream(t *testing.T) {
	got, _ := relayed(t, native.Message{URL: "https://cdn.example/a.zip"})
	if _, ok := got["stream"]; ok {
		t.Fatalf("file relayed as stream: %+v", got)
	}
}
