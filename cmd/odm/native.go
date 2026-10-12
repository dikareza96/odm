package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"odm/internal/native"
)

// runNativeHost relays browser native-messaging requests to a running ODM
// instance over its local HTTP API. It is launched by the browser, not by hand.
func runNativeHost(addr string) {
	target := os.Getenv("ODM_ADDR")
	if target == "" {
		target = addr
	}
	base := "http://" + target
	client := &http.Client{Timeout: 10 * time.Second}

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
			return native.Reply{OK: true, ID: info.ID, Message: "queued"}
		}
		return native.Reply{Error: "unknown message type: " + m.Type}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "odm native host:", err)
	}
}
