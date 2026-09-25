// PymovieLcReader reads PyMovie aperture record files (.pymovie).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

const (
	appTitle = "PyMovie LC Reader"
	// appID identifies the app to Fyne, which keys the preferences store on it.
	// Changing it later loses any saved preferences.
	appID = "com.pymovie.lcreader"

	// Preference keys.
	prefLastFolder = "lastFolder" // folder of the last file picked in the Open dialog
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
	body := container.NewStack()

	show := func(path string) {
		body.Objects = []fyne.CanvasObject{buildContent(path)}
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
	w.Resize(fyne.NewSize(700, 500))
	w.ShowAndRun()
}

// buildContent reads the file at path and returns the window content: the header
// fields above a list of the aperture names, or an error message.
func buildContent(path string) fyne.CanvasObject {
	header, records, err := pymoviefile.ReadFile(path)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Reading %s: %v", path, err))
	}

	names := apertureGroupNames(records)
	info := widget.NewForm(
		widget.NewFormItem("File", widget.NewLabel(path)),
		widget.NewFormItem("Source", widget.NewLabel(header.Source)),
		widget.NewFormItem("Obs date", widget.NewLabel(header.ObsDate)),
		widget.NewFormItem("Roi size", widget.NewLabel(fmt.Sprint(header.RoiSize))),
		widget.NewFormItem("Records", widget.NewLabel(fmt.Sprint(len(records)))),
	)
	title := widget.NewLabelWithStyle(fmt.Sprintf("Apertures in an aperture group (%d)", len(names)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	list := widget.NewList(
		func() int { return len(names) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("%d: %s", i+1, names[i]))
		},
	)
	return container.NewBorder(container.NewVBox(info, widget.NewSeparator(), title), nil, nil, nil, list)
}

// apertureGroupNames returns the aperture names of the first aperture group:
// the records written for the first frame (or field), in the order written.
func apertureGroupNames(records []pymoviefile.Record) []string {
	var names []string
	for _, r := range records {
		if r.Frame != records[0].Frame {
			break
		}
		names = append(names, r.Name)
	}
	return names
}
