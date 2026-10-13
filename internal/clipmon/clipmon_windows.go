//go:build windows

// Package clipmon watches the Windows clipboard for downloadable URLs.
package clipmon

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard      = user32.NewProc("OpenClipboard")
	procCloseClipboard     = user32.NewProc("CloseClipboard")
	procGetClipboardData   = user32.NewProc("GetClipboardData")
	procIsFormatAvailable  = user32.NewProc("IsClipboardFormatAvailable")
	procClipboardSeqNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGlobalLock         = kernel32.NewProc("GlobalLock")
	procGlobalUnlock       = kernel32.NewProc("GlobalUnlock")
)

const cfUnicodeText = 13

func readClipboard() string {
	if ok, _, _ := procIsFormatAvailable.Call(cfUnicodeText); ok == 0 {
		return ""
	}
	if ok, _, _ := procOpenClipboard.Call(0); ok == 0 {
		return ""
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(h)

	// CF_UNICODETEXT is NUL-terminated; bound the scan generously.
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(p)), 1<<20))
}

// Watch polls the clipboard and calls onURL once per newly copied downloadable
// URL. It returns when ctx is cancelled.
// fileTypes is read on every clipboard change so edits in Options apply
// without restarting the watcher.
func Watch(ctx context.Context, fileTypes func() []string, onURL func(string)) {
	var last uint32
	seen := map[string]bool{}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			seq, _, _ := procClipboardSeqNumber.Call()
			if uint32(seq) == last {
				continue
			}
			last = uint32(seq)
			if u, ok := ClipboardURL(readClipboard(), fileTypes()...); ok && !seen[u] {
				seen[u] = true
				onURL(u)
			}
		}
	}
}
