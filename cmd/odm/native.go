package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"odm/internal/ipc"
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
	base := ipc.BaseURL
	client := ipc.Client(path, 10*time.Second)

	err := native.Serve(os.Stdin, os.Stdout, func(m native.Message) native.Reply {
		switch m.Type {
		case "ping":
			return native.Reply{OK: true, Message: "odm"}
		case "", "download":
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
			payload := map[string]any{
				"url":         m.URL,
				"dir":         m.Dir,
				"connections": m.Connections,
			}
			if len(headers) > 0 {
				payload["headers"] = headers
			}
			body, _ := json.Marshal(payload)
			resp, err := client.Post(base+"/api/downloads", "application/json", bytes.NewReader(body))
			if err != nil {
				return native.Reply{Error: "ODM is not running"}
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				return native.Reply{Error: fmt.Sprintf("ODM returned %s", resp.Status)}
			}
			var info struct {
				ID       string `json:"id"`
				FileName string `json:"file_name"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&info)
			msg := "queued"
			if resp.StatusCode == http.StatusOK { // 200, not 201: it was already in the list
				msg = "already in the list"
			}
			return native.Reply{OK: true, ID: info.ID, Message: msg}
		}
		return native.Reply{Error: "unknown message type: " + m.Type}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "odm native host:", err)
	}
}
