//go:build windows

package clipmon

import (
	"os"
	"testing"
)

// TestReadClipboard reads the real clipboard. It needs a desktop session, so
// it runs only when clip-expected.txt (UTF-8) in the working directory holds
// the text that was put on the clipboard.
func TestReadClipboard(t *testing.T) {
	want, err := os.ReadFile("clip-expected.txt")
	if err != nil {
		t.Skip("no clip-expected.txt; this test needs a prepared clipboard")
	}
	if got := readClipboard(); got != string(want) {
		t.Fatalf("readClipboard() = %q, want %q", got, want)
	}
}
