//go:build !windows

package sysproxy

// readSystem has nothing beyond the environment variables on macOS and
// Linux desktops that Go can read without a desktop-specific API.
func readSystem() Config { return Config{} }
