//go:build windows

package launcher

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

// CenterProcessWindow locates the game window for pid and centers it on the
// primary monitor, position only (the game keeps its own size).
func CenterProcessWindow(pid int, fullscreen bool) {
	if fullscreen {
		return
	}
	go func() {
		screenW, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
		screenH, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
		if screenW == 0 || screenH == 0 {
			return
		}

		procGetClassName := user32.NewProc("GetClassNameW")
		var foundHwnd uintptr
		cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
			var wndPid uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wndPid)))
			if int(wndPid) != pid {
				return 1 // continue
			}
			if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
				return 1
			}
			var nameBuf [256]uint16
			procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(len(nameBuf)))
			className := syscall.UTF16ToString(nameBuf[:])
			if className != "GLFW30" && className != "LWJGL" {
				return 1 // keep looking for the real game window
			}
			foundHwnd = hwnd
			return 0 // stop enumeration
		})

		// wait up to 120s for the game window to appear
		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) {
			foundHwnd = 0
			procEnumWindows.Call(cb, 0)
			if foundHwnd != 0 {
				var r winRect
				procGetWindowRect.Call(foundHwnd, uintptr(unsafe.Pointer(&r)))
				w := r.Right - r.Left
				h := r.Bottom - r.Top
				if w > 100 && h > 100 {
					x := (int32(screenW) - w) / 2
					y := (int32(screenH) - h) / 2
					if x < 0 { x = 0 }
					if y < 0 { y = 0 }

					// SWP_NOSIZE | SWP_NOZORDER: move only
					procSetWindowPos.Call(foundHwnd, 0, uintptr(x), uintptr(y), 0, 0, uintptr(0x0001|0x0004))
					
					// Re-assert one more time after a short delay since Minecraft might reset it initially
					time.Sleep(3 * time.Second)
					procSetWindowPos.Call(foundHwnd, 0, uintptr(x), uintptr(y), 0, 0, uintptr(0x0001|0x0004))
					return
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
}
