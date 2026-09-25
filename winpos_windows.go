//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// Fyne has no API for a window's position, so on Windows it is read and set
// through the native window handle.

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procIsIconic        = user32.NewProc("IsIconic")
	procIsZoomed        = user32.NewProc("IsZoomed")
	procMonitorFromRect = user32.NewProc("MonitorFromRect")
)

type winRect struct{ Left, Top, Right, Bottom int32 }

const (
	swpNoSize            = 0x0001
	swpNoZOrder          = 0x0004
	swpNoActivate        = 0x0010
	monitorDefaultToNull = 0
)

// hwnd returns w's native window handle, or 0 if it has none yet.
func hwnd(w fyne.Window) uintptr {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var h uintptr
	nw.RunNative(func(ctx any) {
		if c, ok := ctx.(driver.WindowsWindowContext); ok {
			h = c.HWND
		}
	})
	return h
}

// windowPosition returns the screen position of w's top-left corner in pixels.
// ok is false if it is unknown, or the window is minimized or maximized.
func windowPosition(w fyne.Window) (x, y int, ok bool) {
	h := hwnd(w)
	if h == 0 {
		return 0, 0, false
	}
	if r, _, _ := procIsIconic.Call(h); r != 0 {
		return 0, 0, false
	}
	if r, _, _ := procIsZoomed.Call(h); r != 0 {
		return 0, 0, false
	}
	var rc winRect
	if r, _, _ := procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return 0, 0, false
	}
	return int(rc.Left), int(rc.Top), true
}

// setWindowPosition moves w's top-left corner to (x, y), unless the top of the
// window there would not be on any monitor.
func setWindowPosition(w fyne.Window, x, y int) {
	h := hwnd(w)
	if h == 0 {
		return
	}
	titleBar := winRect{Left: int32(x), Top: int32(y), Right: int32(x) + 100, Bottom: int32(y) + 30}
	if m, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&titleBar)), monitorDefaultToNull); m == 0 {
		return
	}
	procSetWindowPos.Call(h, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
}
