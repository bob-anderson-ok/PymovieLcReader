package main

import "fyne.io/fyne/v2"

// Windows keep their size, and on Windows their position, in the preferences
// under a key: key+".w" and key+".h" hold the content size, key+".x" and
// key+".y" the screen position of the window's top-left corner in pixels.

// restoreWindowSize resizes w to the size saved under key, or to def.
func restoreWindowSize(prefs fyne.Preferences, w fyne.Window, key string, def fyne.Size) {
	w.Resize(fyne.NewSize(
		float32(prefs.FloatWithFallback(key+".w", float64(def.Width))),
		float32(prefs.FloatWithFallback(key+".h", float64(def.Height))),
	))
}

// restoreWindowPosition moves w to the position saved under key. It only works
// once the window has been shown, and does nothing if no position was saved or
// the saved position is no longer on a screen.
func restoreWindowPosition(prefs fyne.Preferences, w fyne.Window, key string) {
	x, y := prefs.IntWithFallback(key+".x", noPosition), prefs.IntWithFallback(key+".y", noPosition)
	if x != noPosition && y != noPosition {
		setWindowPosition(w, x, y)
	}
}

// saveWindowGeometry saves w's size and position under key. Call it before
// the window closes.
func saveWindowGeometry(prefs fyne.Preferences, w fyne.Window, key string) {
	x, y, ok := windowPosition(w)
	if !ok { // minimized or maximized: keep the last normal geometry
		return
	}
	size := w.Canvas().Size()
	prefs.SetFloat(key+".w", float64(size.Width))
	prefs.SetFloat(key+".h", float64(size.Height))
	prefs.SetInt(key+".x", x)
	prefs.SetInt(key+".y", y)
}

const noPosition = -1 << 31
