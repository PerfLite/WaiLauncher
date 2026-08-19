//go:build !windows

package launcher

// CenterProcessWindow is a no-op on non-Windows platforms.
func CenterProcessWindow(pid int, fullscreen bool) {}
