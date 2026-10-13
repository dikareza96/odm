//go:build windows

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
	procGlobalSize         = kernel32.NewProc("GlobalSize")
)

const cfUnicodeText = 13

// Native reports whether this platform has a built-in watcher (Watch); where
// it does not, the desktop UI polls through Fyne instead.
const Native = true

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
	size, _, _ := procGlobalSize.Call(h)
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 || size < 2 {
		return ""
	}
	defer procGlobalUnlock.Call(h)

	// The block lives in the Windows heap, not Go's. unsafe.Add from nil
	// turns the address into a pointer without a uintptr->Pointer
	// conversion (which vet cannot tell apart from a dangling Go pointer),
	// and the slice is bounded by GlobalSize instead of an arbitrary cap,
	// so a block without a terminating NUL is never read past its end.
	text := unsafe.Slice((*uint16)(unsafe.Add(nil, p)), size/2)
	return syscall.UTF16ToString(text) // stops at the first NUL
}

// Watch checks the clipboard's change counter every 500 ms and reads the
// text only when it changed, queueing new downloadable URLs until ctx ends.
func Watch(ctx context.Context, cfg func() Config, onURL func(string)) {
	var t Tracker
	t.Next(readClipboard(), Config{}) // baseline: ignore what is there already
	last, _, _ := procClipboardSeqNumber.Call()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			seq, _, _ := procClipboardSeqNumber.Call()
			if seq == last {
				continue
			}
			last = seq
			if u, ok := t.Next(readClipboard(), cfg()); ok {
				onURL(u)
			}
		}
	}
}
