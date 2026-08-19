//go:build windows

package launcher

import "syscall"

// hideWindowAttr keeps helper java processes (installer processors) from
// flashing console windows on screen.
func hideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
