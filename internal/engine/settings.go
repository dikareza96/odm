package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Settings are the user-facing options persisted between runs.
type Settings struct {
	Dir         string `json:"dir"`
	Connections int    `json:"connections"`
	Concurrent  int    `json:"concurrent"`
	SpeedLimit  int64  `json:"speed_limit"`
	Categories  bool   `json:"categories"`
	Clipboard   bool   `json:"clipboard"`
}

// DefaultSettings returns settings for a fresh install.
func DefaultSettings(dir string) Settings {
	return Settings{
		Dir:         dir,
		Connections: DefaultConnections,
		Concurrent:  DefaultFileLimit,
		Categories:  true,
	}
}

func (s *Settings) normalize(fallbackDir string) {
	if strings.TrimSpace(s.Dir) == "" {
		s.Dir = fallbackDir
	}
	if s.Connections <= 0 {
		s.Connections = DefaultConnections
	}
	if s.Connections > 32 {
		s.Connections = 32
	}
	if s.Concurrent <= 0 {
		s.Concurrent = DefaultFileLimit
	}
	if s.SpeedLimit < 0 {
		s.SpeedLimit = 0
	}
}

// LoadSettings reads settings from path, falling back to defaults.
func LoadSettings(path, fallbackDir string) Settings {
	s := DefaultSettings(fallbackDir)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	s.normalize(fallbackDir)
	return s
}

// SaveSettings writes settings to path.
func SaveSettings(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var categoryExt = map[string]string{
	".mp4": "Video", ".mkv": "Video", ".avi": "Video", ".mov": "Video", ".webm": "Video",
	".flv": "Video", ".wmv": "Video", ".m4v": "Video", ".ts": "Video", ".mpg": "Video",
	".mpeg": "Video", ".3gp": "Video", ".vob": "Video", ".mts": "Video",

	".mp3": "Audio", ".m4a": "Audio", ".aac": "Audio", ".flac": "Audio", ".wav": "Audio",
	".ogg": "Audio", ".wma": "Audio", ".opus": "Audio", ".aiff": "Audio",

	".pdf": "Documents", ".doc": "Documents", ".docx": "Documents", ".xls": "Documents",
	".xlsx": "Documents", ".ppt": "Documents", ".pptx": "Documents", ".txt": "Documents",
	".epub": "Documents", ".csv": "Documents", ".rtf": "Documents", ".odt": "Documents",

	".zip": "Compressed", ".rar": "Compressed", ".7z": "Compressed", ".tar": "Compressed",
	".gz": "Compressed", ".bz2": "Compressed", ".xz": "Compressed", ".zst": "Compressed",

	".exe": "Programs", ".msi": "Programs", ".dmg": "Programs", ".pkg": "Programs",
	".deb": "Programs", ".rpm": "Programs", ".appx": "Programs", ".apk": "Programs",
	".bat": "Programs", ".sh": "Programs",

	".jpg": "Images", ".jpeg": "Images", ".png": "Images", ".gif": "Images", ".webp": "Images",
	".bmp": "Images", ".svg": "Images", ".tiff": "Images", ".ico": "Images", ".heic": "Images",

	".iso": "Disk Images", ".img": "Disk Images", ".vhd": "Disk Images",
}

// Category maps a file name to its download subfolder.
func Category(name string) string {
	if c, ok := categoryExt[strings.ToLower(filepath.Ext(name))]; ok {
		return c
	}
	return "Other"
}

// categoryOrder is the presentation order of the category folders.
var categoryOrder = []string{
	"Video", "Audio", "Documents", "Programs", "Compressed", "Images", "Disk Images", "Other",
}

// Categories returns the known category folder names, in display order.
func Categories() []string {
	out := make([]string, len(categoryOrder))
	copy(out, categoryOrder)
	return out
}

// KnownFileType reports whether name has an extension ODM recognises.
func KnownFileType(name string) bool {
	_, ok := categoryExt[strings.ToLower(filepath.Ext(name))]
	return ok
}
