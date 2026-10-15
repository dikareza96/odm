package engine

import "testing"

func TestSanitizeNameWindowsDevices(t *testing.T) {
	cases := map[string]string{
		"CON":         "_CON",
		"nul.mp4":     "_nul.mp4",
		"Com1.tar.gz": "_Com1.tar.gz",
		"LPT9 .txt":   "_LPT9 .txt",
		"COM²":        "_COM²",
		"conin$.log":  "_conin$.log",
		"CONSOLE.mp4": "CONSOLE.mp4",
		"COM10.mp4":   "COM10.mp4",
		"my nul.mp4":  "my nul.mp4",
		"aux_video":   "aux_video",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
