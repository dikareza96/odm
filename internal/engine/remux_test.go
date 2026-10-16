package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"odm/internal/media"
)

// fakeFFmpeg writes a shell script standing in for ffmpeg: "ok" copies
// standard input (the joined MPEG-TS) to the output (the last argument)
// with a marker, "fail" exits with an error.
func fakeFFmpeg(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as ffmpeg")
	}
	bin := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nfor a; do last=$a; done\n{ printf 'MP4:'; cat; } > \"$last\"\n"
	if mode == "fail" {
		script = "#!/bin/sh\necho 'non monotonically increasing dts' >&2\nexit 1\n"
	}
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	setFFmpegPath(bin)
	t.Cleanup(func() { setFFmpegPath(noFFmpeg) })
	return bin
}

func TestStreamRemuxedToMP4(t *testing.T) {
	fakeFFmpeg(t, "ok")
	srv := segServer(t, threeSegs, nil)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "MP4:[/seg0.ts][/seg1.ts][/seg2.ts]" {
		t.Fatalf("output %q", got)
	}
	if info := d.Info(); info.FileName != "clip.mp4" || info.Error != "" {
		t.Fatalf("info %q %q", info.FileName, info.Error)
	}
}

func TestStreamRemuxFailsKeepsTS(t *testing.T) {
	fakeFFmpeg(t, "fail")
	srv := segServer(t, threeSegs, nil)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	d.Start(context.Background())
	if got := string(readResult(t, d)); got != "[/seg0.ts][/seg1.ts][/seg2.ts]" {
		t.Fatalf("output %q", got)
	}
	info := d.Info()
	if info.FileName != "clip.ts" || !strings.Contains(info.Error, "MPEG-TS") || !strings.Contains(info.Error, "dts") {
		t.Fatalf("info %q %q", info.FileName, info.Error)
	}
	if d.Stream.WorkName != "clip.mp4" {
		t.Fatalf("work folder name not kept: %q", d.Stream.WorkName)
	}
	if _, err := os.Stat(filepath.Join(info.Dir, "clip.mp4.odmpart")); !os.IsNotExist(err) {
		t.Fatal("the .mp4 claim was left behind")
	}
	if _, err := os.Stat(filepath.Join(info.Dir, "clip.mp4.odmparts")); !os.IsNotExist(err) {
		t.Fatal("work folder left behind")
	}
}

// ffmpeg found when the file was named, gone when it is joined.
func TestStreamRemuxFFmpegGone(t *testing.T) {
	bin := fakeFFmpeg(t, "ok")
	srv := segServer(t, threeSegs, nil)
	d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	tracks, err := d.loadStream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.prepareStream(tracks); err != nil {
		t.Fatal(err)
	}
	if d.FileName != "clip.mp4" {
		t.Fatalf("named %q", d.FileName)
	}
	os.Remove(bin)
	d.Start(context.Background())
	if info := d.Info(); info.FileName != "clip.ts" || !strings.Contains(info.Error, "MPEG-TS") {
		t.Fatalf("info %q %q %s", info.FileName, info.Error, info.Status)
	}
}

// With a real ffmpeg: an HLS stream of MPEG-TS segments comes out as a
// playable MP4.
func TestStreamRemuxRealFFmpeg(t *testing.T) {
	bin := os.Getenv("ODM_TEST_FFMPEG")
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("ffmpeg"); err != nil {
			t.Skip("no ffmpeg")
		}
	}
	src := t.TempDir()
	cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=4:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=duration=4",
		"-c:v", "libx264", "-g", "10", "-c:a", "aac",
		"-f", "hls", "-hls_time", "1", "-hls_list_size", "0", "-hls_playlist_type", "vod",
		"p.m3u8")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot generate HLS input: %v %s", err, out)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(src)))
	defer srv.Close()

	setFFmpegPath(bin)
	defer setFFmpegPath(noFFmpeg)
	d := newStreamDownload(t, srv.Client(), 4, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
	d.Start(context.Background())
	info := d.Info()
	if info.Status != StatusComplete || info.FileName != "clip.mp4" || info.Error != "" {
		t.Fatalf("%s %s %q", info.Status, info.FileName, info.Error)
	}
	probe, _ := exec.Command(bin, "-hide_banner", "-i", filepath.Join(info.Dir, info.FileName)).CombinedOutput()
	s := string(probe)
	if !strings.Contains(s, "mp4") || !strings.Contains(s, "Video: h264") || !strings.Contains(s, "Audio: aac") {
		t.Fatalf("remuxed file:\n%s", s)
	}
}

// TestStreamRemuxTimestampJumpRealFFmpeg covers recordings whose timestamps
// jump: an ad break or encoder restart (they start again) and a recording
// resumed after a gap (they leap minutes ahead). ffmpeg's MPEG-TS reader
// takes such jumps as discontinuities and shifts them out, so the MP4 plays
// straight through with video and audio together; this keeps that so.
func TestStreamRemuxTimestampJumpRealFFmpeg(t *testing.T) {
	bin := os.Getenv("ODM_TEST_FFMPEG")
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("ffmpeg"); err != nil {
			t.Skip("no ffmpeg")
		}
	}
	src := t.TempDir()
	encode := func(name, pattern, offset string) {
		cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", pattern+"=duration=3:size=160x120:rate=25",
			"-f", "lavfi", "-i", "sine=duration=3",
			"-c:v", "libx264", "-g", "25", "-c:a", "aac", "-output_ts_offset", offset, "-f", "mpegts", name)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("cannot generate MPEG-TS input: %v %s", err, out)
		}
	}
	encode("a.ts", "testsrc", "0")
	encode("restart.ts", "testsrc2", "0") // starts again at 0
	encode("later.ts", "testsrc2", "300") // five minutes ahead
	setFFmpegPath(bin)
	defer setFFmpegPath(noFFmpeg)

	for _, second := range []string{"restart.ts", "later.ts"} {
		playlist := "#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:3,\na.ts\n#EXTINF:3,\n#EXT-X-DISCONTINUITY\n" + second + "\n#EXT-X-ENDLIST\n"
		os.WriteFile(filepath.Join(src, "p.m3u8"), []byte(playlist), 0o644)
		srv := httptest.NewServer(http.FileServer(http.Dir(src)))
		d := newStreamDownload(t, srv.Client(), 2, StreamJob{Kind: media.KindHLS, Manifest: srv.URL + "/p.m3u8"})
		d.Start(context.Background())
		srv.Close()
		info := d.Info()
		if info.Status != StatusComplete || info.FileName != "clip.mp4" || info.Error != "" {
			t.Fatalf("%s: %s %s %q", second, info.Status, info.FileName, info.Error)
		}
		spans := streamSpans(t, bin, filepath.Join(info.Dir, info.FileName))
		if len(spans) != 2 {
			t.Fatalf("%s: streams %v", second, spans)
		}
		for i, s := range spans {
			if s[1]-s[0] < 5.5 || s[1]-s[0] > 6.5 {
				t.Errorf("%s: stream %d spans %.2f-%.2f s, want about 6 s", second, i, s[0], s[1])
			}
		}
		if d := spans[0][1] - spans[1][1]; d > 0.2 || d < -0.2 {
			t.Errorf("%s: video ends at %.3f s, audio at %.3f s", second, spans[0][1], spans[1][1])
		}
	}
}

// streamSpans returns each stream's first and last time in seconds, read
// from ffmpeg's framemd5 listing ("#tb N: num/den" headers, then rows of
// "stream, dts, pts, duration, ...").
func streamSpans(t *testing.T, bin, file string) [][2]float64 {
	t.Helper()
	out, err := exec.Command(bin, "-hide_banner", "-loglevel", "error", "-i", file, "-c", "copy", "-f", "framemd5", "-").Output()
	if err != nil {
		t.Fatalf("framemd5: %v", err)
	}
	tb := map[int]float64{}
	var spans [][2]float64
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "#tb ") {
			var s, n, d int
			fmt.Sscanf(line, "#tb %d: %d/%d", &s, &n, &d)
			tb[s] = float64(n) / float64(d)
			continue
		}
		if line == "" || line[0] == '#' {
			continue
		}
		var s int
		var dts, pts, dur int64
		fmt.Sscanf(strings.ReplaceAll(line, " ", ""), "%d,%d,%d,%d", &s, &dts, &pts, &dur)
		for len(spans) <= s {
			spans = append(spans, [2]float64{1e18, 0})
		}
		start, end := float64(pts)*tb[s], float64(pts+dur)*tb[s]
		spans[s][0] = min(spans[s][0], start)
		spans[s][1] = max(spans[s][1], end)
	}
	return spans
}
