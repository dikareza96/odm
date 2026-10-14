package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"odm/internal/ipc"
	"odm/internal/media"
	"odm/internal/native"
)

// runNativeHost relays browser native-messaging requests to a running ODM
// instance over its control socket. It is launched by the browser, not by
// hand, as the same user, so it resolves the same config directory;
// ODM_SOCKET overrides the socket path.
func runNativeHost(configDir string) {
	path := os.Getenv("ODM_SOCKET")
	if path == "" {
		path = ipc.SocketPath(configDir)
	}
	client := ipc.Client(path, 10*time.Second)

	err := native.Serve(os.Stdin, os.Stdout, func(m native.Message) native.Reply {
		switch m.Type {
		case "ping":
			return native.Reply{OK: true, Message: "odm"}
		case "config":
			return relayConfig(client)
		case "", "download":
			return relayDownload(client, m)
		}
		return native.Reply{Error: "unknown message type: " + m.Type}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "odm native host:", err)
	}
}

// relayDownload hands a browser download to ODM. The extension cancels the
// browser's own download only when this replies OK, so every failure must
// come back as an error rather than a silent success.
func relayDownload(client *http.Client, m native.Message) native.Reply {
	if m.URL == "" {
		return native.Reply{Error: "missing url"}
	}
	headers := map[string]string{}
	for k, v := range m.Headers {
		headers[k] = v
	}
	if m.Referrer != "" {
		headers["Referer"] = m.Referrer
	}
	if m.Cookies != "" {
		headers["Cookie"] = m.Cookies
	}
	// Some servers only answer the browser that started the session.
	if m.UserAgent != "" {
		headers["User-Agent"] = m.UserAgent
	}
	payload := map[string]any{
		"url":         m.URL,
		"dir":         m.Dir,
		"connections": m.Connections,
		"file_name":   native.SuggestName(m.Filename, m.Title, m.Mime),
		"prompt":      true, // a person clicked this in the browser
	}
	if len(headers) > 0 {
		payload["headers"] = headers
	}
	if m.Kind == "stream" {
		// The extension only saw the response headers, not the body, so
		// the format comes from the Content-Type and the URL.
		k := media.Detect(m.Mime, m.URL, nil)
		if k == media.KindFile || k == media.KindNone {
			// The extension trusts a playlist extension over a media type
			// some servers mislabel playlists with (video/mp2t, ...).
			k = media.Detect("", m.URL, nil)
		}
		if k != media.KindHLS && k != media.KindDASH {
			return native.Reply{Error: "this stream format is not supported"}
		}
		payload["stream"] = string(k)
		payload["file_name"] = native.StreamName(m.Title)
	}
	body, _ := json.Marshal(payload)
	resp, err := client.Post(ipc.BaseURL+"/api/downloads", "application/json", bytes.NewReader(body))
	if err != nil {
		return native.Reply{Error: "ODM is not running"}
	}
	defer resp.Body.Close()
	var info struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&info)
	switch resp.StatusCode {
	case http.StatusAccepted:
		return native.Reply{OK: true, Message: "opened in ODM"}
	case http.StatusCreated:
		return native.Reply{OK: true, ID: info.ID, Message: "queued"}
	case http.StatusOK:
		return native.Reply{OK: true, ID: info.ID, Message: "already in the list"}
	}
	return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
}

// relayConfig returns the settings the extension applies itself.
func relayConfig(client *http.Client) native.Reply {
	resp, err := client.Get(ipc.BaseURL + "/api/settings")
	if err != nil {
		return native.Reply{Error: "ODM is not running"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
	}
	var st struct {
		FileTypes    []string `json:"file_types"`
		ExcludeSites []string `json:"exclude_sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return native.Reply{Error: "unreadable settings: " + err.Error()}
	}
	if st.ExcludeSites == nil {
		st.ExcludeSites = []string{}
	}
	return native.Reply{OK: true, Config: &native.Config{FileTypes: st.FileTypes, ExcludeSites: st.ExcludeSites}}
}
