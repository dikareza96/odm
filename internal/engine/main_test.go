package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// noFFmpeg is the ffmpeg path tests run with unless they set one: a path
// that does not exist, so whether the machine (or CI runner) has ffmpeg
// does not change file names (.ts vs .mp4) or results.
var noFFmpeg string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "odm-engine-test")
	if err != nil {
		panic(err)
	}
	noFFmpeg = filepath.Join(dir, "no-ffmpeg-here")
	setFFmpegPath(noFFmpeg)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
