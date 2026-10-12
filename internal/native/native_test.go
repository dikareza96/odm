package native

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := Message{Type: "download", URL: "https://host/file.zip", Referrer: "https://host/page"}
	if err := WriteMessage(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != want.URL || got.Type != want.Type || got.Referrer != want.Referrer {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestReadRejectsEmptyAndOversize(t *testing.T) {
	var zero [4]byte
	if _, err := ReadMessage(bytes.NewReader(zero[:])); err == nil {
		t.Fatal("expected error for empty frame")
	}

	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], maxMessage+1)
	if _, err := ReadMessage(bytes.NewReader(hdr[:])); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestServeHandlesMultipleThenEOF(t *testing.T) {
	var in, out bytes.Buffer
	_ = WriteMessage(&in, Message{Type: "ping"})
	_ = WriteMessage(&in, Message{Type: "download", URL: "https://host/a.iso"})

	var seen []string
	if err := Serve(&in, &out, func(m Message) Reply {
		seen = append(seen, m.Type)
		return Reply{OK: true, ID: "x"}
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "ping" || seen[1] != "download" {
		t.Fatalf("handled %v", seen)
	}
}

func TestServeRepliesAreFramedJSON(t *testing.T) {
	var in, out bytes.Buffer
	_ = WriteMessage(&in, Message{Type: "download", URL: "https://host/a.zip"})
	_ = Serve(&in, &out, func(Message) Reply { return Reply{OK: true, ID: "abc"} })

	raw := out.Bytes()
	if len(raw) < 4 {
		t.Fatal("no framed output")
	}
	n := binary.LittleEndian.Uint32(raw[:4])
	if int(n) != len(raw)-4 {
		t.Fatalf("length prefix %d does not match payload %d", n, len(raw)-4)
	}
	var reply Reply
	if err := json.Unmarshal(raw[4:], &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.OK || reply.ID != "abc" {
		t.Fatalf("reply = %+v", reply)
	}
}
