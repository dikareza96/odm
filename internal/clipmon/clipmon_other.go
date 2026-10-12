//go:build !windows

package clipmon

import "context"

// Watch is a no-op outside Windows; the caller keeps its context alive.
func Watch(ctx context.Context, onURL func(string)) {
	<-ctx.Done()
}
