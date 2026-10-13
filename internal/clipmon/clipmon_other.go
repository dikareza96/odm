//go:build !windows

package clipmon

import "context"

// Native reports whether this platform has a built-in watcher; here the
// desktop UI polls the clipboard through Fyne (Poll) instead.
const Native = false

// Watch is a no-op without a native clipboard API; it returns when ctx ends.
func Watch(ctx context.Context, _ func() Config, _ func(string)) {
	<-ctx.Done()
}
