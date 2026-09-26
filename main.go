// PymovieLcReader reads PyMovie aperture record files (.pymovie).
package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	appVersion = "1.1"
	appTitle   = "PyMovie LC Reader " + appVersion
	// appID identifies the app to Fyne, which keys the preferences store on it.
	// Changing it later loses any saved preferences.
	appID = "com.pymovie.lcreader"

	// Preference keys.
	prefLastFolder = "lastFolder" // folder of the last file picked in the Open dialog
	prefDotSize    = "dotSize"    // dot diameter on the light curve plot
	prefMainWindow = "mainWindow" // prefix of the main window's size and position keys
)

func main() {
	// A path on the command line opens that file; otherwise LC-test.pymovie if it is here.
	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else if _, err := os.Stat("LC-test.pymovie"); err == nil {
		path = "LC-test.pymovie"
	}

	a := app.NewWithID(appID)
	w := a.NewWindow(appTitle)
	w.SetMaster() // closing the main window quits, closing the image windows too
	body := container.NewStack()
	var current *viewer // the viewer of the file shown, or nil

	show := func(path string) {
		if current != nil {
			current.closeImageWindows()
		}
		var content fyne.CanvasObject
		content, current = newViewer(path, a, w)
		body.Objects = []fyne.CanvasObject{content}
		body.Refresh()
		w.SetTitle(appTitle + " - " + filepath.Base(path))
	}

	openButton := widget.NewButtonWithIcon("Open .pymovie file...", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil { // cancelled
				return
			}
			reader.Close()
			path = reader.URI().Path()
			a.Preferences().SetString(prefLastFolder, filepath.Dir(path))
			show(path)
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".pymovie"}))
		// Start in the last folder a file was opened from if it still exists,
		// else the folder of the file being shown, else the working directory.
		dir := a.Preferences().String(prefLastFolder)
		if info, err := os.Stat(dir); dir == "" || err != nil || !info.IsDir() {
			dir = "."
			if path != "" {
				dir = filepath.Dir(path)
			}
		}
		if abs, err := filepath.Abs(dir); err == nil {
			if lister, err := storage.ListerForURI(storage.NewFileURI(abs)); err == nil {
				d.SetLocation(lister)
			}
		}
		d.Show()
		d.Resize(w.Canvas().Size()) // after Show: the dialog has no content to size before it
	})

	if path != "" {
		show(path)
	} else {
		body.Objects = []fyne.CanvasObject{widget.NewLabel("Use the Open button to choose a .pymovie file.")}
	}

	w.SetContent(container.NewBorder(container.NewHBox(openButton), nil, nil, nil, body))
	prefs := a.Preferences()
	restoreWindowSize(prefs, w, prefMainWindow, fyne.NewSize(1100, 650))
	w.SetCloseIntercept(func() {
		saveWindowGeometry(prefs, w, prefMainWindow)
		if current != nil {
			current.closeImageWindows() // saves their geometry
		}
		w.Close()
	})
	// Windows have no native window, so no position, until the app is running.
	a.Lifecycle().SetOnStarted(func() {
		restoreWindowPosition(prefs, w, prefMainWindow)
		if current != nil {
			current.restoreImageWindowPositions()
		}
	})
	w.ShowAndRun()
}
