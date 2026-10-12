package engine

import (
	"path/filepath"
	"testing"
)

func TestCategoryMapping(t *testing.T) {
	cases := map[string]string{
		"movie.MP4":       "Video",
		"song.flac":       "Audio",
		"report.pdf":      "Documents",
		"archive.tar.gz":  "Compressed",
		"setup.exe":       "Programs",
		"photo.jpeg":      "Images",
		"ubuntu.iso":      "Disk Images",
		"data.unknownext": "Other",
		"noext":           "Other",
	}
	for name, want := range cases {
		if got := Category(name); got != want {
			t.Errorf("Category(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := Settings{}
	s.normalize("/fallback")
	if s.Dir != "/fallback" {
		t.Errorf("dir fallback = %q", s.Dir)
	}
	if s.Connections != DefaultConnections {
		t.Errorf("connections = %d", s.Connections)
	}
	if s.Concurrent != DefaultFileLimit {
		t.Errorf("concurrent = %d", s.Concurrent)
	}

	big := Settings{Dir: "/d", Connections: 99, Concurrent: -1, SpeedLimit: -5}
	big.normalize("/fallback")
	if big.Connections != 32 {
		t.Errorf("connections cap = %d", big.Connections)
	}
	if big.Concurrent != DefaultFileLimit {
		t.Errorf("concurrent = %d", big.Concurrent)
	}
	if big.SpeedLimit != 0 {
		t.Errorf("speed limit = %d", big.SpeedLimit)
	}
}

func TestLoadSaveSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	def := LoadSettings(path, "/fallback")
	if def.Dir != "/fallback" || def.Connections != DefaultConnections {
		t.Fatalf("defaults wrong: %+v", def)
	}

	want := Settings{Dir: "/downloads", Connections: 12, Concurrent: 5, SpeedLimit: 1048576, Categories: false}
	if err := SaveSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings(path, "/fallback")
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}
