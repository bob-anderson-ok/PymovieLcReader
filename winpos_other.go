//go:build !windows

package main

import "fyne.io/fyne/v2"

// Window positions are only kept on Windows; elsewhere only sizes are.

func windowPosition(fyne.Window) (x, y int, ok bool) { return 0, 0, true }

func setWindowPosition(fyne.Window, int, int) {}
