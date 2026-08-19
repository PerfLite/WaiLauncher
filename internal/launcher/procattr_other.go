//go:build !windows

package launcher

import "syscall"

// hideWindowAttr is a no-op off Windows (console windows are not spawned).
func hideWindowAttr() *syscall.SysProcAttr {
	return nil
}
