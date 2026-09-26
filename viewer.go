package main

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

const defaultDotSize = 4.0

// curveColors are the starting colors of the light curves, in aperture order.
var curveColors = []color.Color{
	color.NRGBA{R: 0x1f, G: 0x77, B: 0xb4, A: 0xff}, // blue
	color.NRGBA{R: 0xd6, G: 0x27, B: 0x28, A: 0xff}, // red
	color.NRGBA{R: 0x2c, G: 0xa0, B: 0x2c, A: 0xff}, // green
	color.NRGBA{R: 0xff, G: 0x7f, B: 0x0e, A: 0xff}, // orange
	color.NRGBA{R: 0x94, G: 0x67, B: 0xbd, A: 0xff}, // purple
	color.NRGBA{R: 0x8c, G: 0x56, B: 0x4b, A: 0xff}, // brown
	color.NRGBA{R: 0xe3, G: 0x77, B: 0xc2, A: 0xff}, // pink
	color.NRGBA{R: 0x17, G: 0xbe, B: 0xcf, A: 0xff}, // cyan
}

// viewer shows one .pymovie file: its header, controls for choosing and
// styling light curves, and the plot.
type viewer struct {
	app   fyne.App
	win   fyne.Window
	prefs fyne.Preferences

	curves   []*lightCurve
	selected []bool
	colors   []color.Color
	ticker   *timeTicker

	allXMin, allXMax float64 // first and last frame in the file
	xMin, xMax       float64 // begin and end frames shown, as entered

	useAppsum  bool
	yMin, yMax float64
	dotSize    float64
	redLevel   float64 // image pixels at or above this are red; +Inf for none
	saturation uint16  // the records' saturation value, the default red level; 0 if not set

	frames []float64 // every frame, sorted: the points the cursor steps through
	cursor int       // index into frames

	checks    []*widget.Check // checks[i] selects curves[i]
	imageWins []*imageWindow  // imageWins[i] shows curves[i]'s images; nil when closed

	path     string
	header   pymoviefile.Header
	frameWin fyne.Window // shows the initial frame; nil when closed

	yMinEntry, yMaxEntry *widget.Entry
	plot                 *plotWidget
	readout              *widget.Label
}

// newViewer reads the file at path and returns the window content for it, and
// the viewer, which opens an image window for each selected aperture. If the
// file can't be read, the content is an error message and the viewer is nil.
func newViewer(path string, app fyne.App, win fyne.Window) (fyne.CanvasObject, *viewer) {
	win.Canvas().SetOnTypedKey(nil) // drop the previous file's cursor keys
	header, records, err := pymoviefile.ReadFile(path)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Reading %s: %v", path, err)), nil
	}

	v := &viewer{
		path:       path,
		header:     header,
		app:        app,
		win:        win,
		prefs:      app.Preferences(),
		curves:     buildLightCurves(records),
		dotSize:    app.Preferences().FloatWithFallback(prefDotSize, defaultDotSize),
		redLevel:   math.Inf(1),
		saturation: saturationLevel(records),
	}
	v.selected = make([]bool, len(v.curves))
	v.imageWins = make([]*imageWindow, len(v.curves))
	for i := range v.curves {
		v.colors = append(v.colors, curveColors[i%len(curveColors)])
	}
	if len(v.selected) > 0 {
		v.selected[0] = true
	}
	v.ticker = newTimeTicker(v.curves)
	v.allXMin, v.allXMax = frameRange(v.curves)
	v.frames = allFrames(v.curves)

	v.plot = newPlotWidget(v.settings, win.Canvas())
	v.plot.onTap = func(frame float64) {
		if len(v.frames) > 0 {
			v.setCursor(nearestIndex(v.frames, frame))
		}
	}
	v.plot.onKey = v.cursorKey
	// The arrow keys also move the cursor when nothing has the focus, such as
	// before the plot has been clicked.
	win.Canvas().SetOnTypedKey(v.cursorKey)
	v.readout = widget.NewLabel("")

	info := widget.NewForm(
		widget.NewFormItem("Source", truncatedLabel(header.Source)),
		widget.NewFormItem("Obs date", widget.NewLabel(header.ObsDate)),
		widget.NewFormItem("Records", widget.NewLabel(fmt.Sprint(len(records)))),
		widget.NewFormItem("Frame size", widget.NewLabel(frameSizeText(header))),
	)
	frameButton := widget.NewButton("Show initial frame", v.openFrameWindow)
	if !hasInitialFrame(header) {
		frameButton.Disable() // written before PyMovie recorded the initial frame
	}

	controls := container.NewVBox(
		info,
		frameButton,
		widget.NewSeparator(),
		boldLabel("Apertures"),
		v.apertureRows(),
		widget.NewSeparator(),
		v.frameRangeControls(),
		widget.NewSeparator(),
		v.fieldCheck(),
		v.limitControls(),
		widget.NewSeparator(),
		v.dotSizeControl(),
		widget.NewSeparator(),
		v.redLevelControl(),
	)
	v.resetLimits()
	v.setCursor(0)
	for i, on := range v.selected {
		if on {
			v.openImageWindow(i)
		}
	}
	if hasInitialFrame(header) {
		v.openFrameWindow()
	}

	plotPane := container.NewBorder(nil, v.readout, nil, nil, v.plot)
	split := container.NewHSplit(container.NewVScroll(controls), plotPane)
	split.Offset = 0.3
	return split, v
}

// settings returns the current plot settings.
func (v *viewer) settings() plotSettings {
	xMin, xMax := v.frameRange()
	s := plotSettings{
		UseAppsum: v.useAppsum,
		XMin:      xMin,
		XMax:      xMax,
		YMin:      v.yMin,
		YMax:      v.yMax,
		DotSize:   v.dotSize,
		Ticker:    v.ticker,
	}
	if len(v.frames) > 0 {
		s.ShowCursor, s.Cursor = true, v.frames[v.cursor]
	}
	for i, c := range v.curves {
		if v.selected[i] {
			s.Curves = append(s.Curves, c)
			s.Colors = append(s.Colors, v.colors[i])
		}
	}
	return s
}

func (v *viewer) redraw() { v.plot.redraw() }

// cursorKey moves the cursor one point left or right for the arrow keys.
func (v *viewer) cursorKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyLeft:
		v.setCursor(v.cursor - 1)
	case fyne.KeyRight:
		v.setCursor(v.cursor + 1)
	}
}

// setCursor moves the cursor to frames[i], kept within the frames, and shows
// the frame, its timestamp and the selected curves' values there.
func (v *viewer) setCursor(i int) {
	if len(v.frames) == 0 {
		return
	}
	first, last := v.visibleFrames()
	v.cursor = max(first, min(i, last))
	frame := v.frames[v.cursor]

	text := fmt.Sprintf("Frame %g", frame)
	if v.ticker != nil {
		text += "   " + v.ticker.stampAt(frame)
	}
	for i, c := range v.curves {
		if !v.selected[i] {
			continue
		}
		if value, ok := c.valueAt(frame, v.useAppsum); ok {
			text += fmt.Sprintf("   %s: %.6g", c.Name, value)
		}
	}
	v.readout.SetText(text)
	v.redraw()
	for _, iw := range v.imageWins {
		if iw != nil {
			iw.showFrame(frame, v.useAppsum)
		}
	}
}

// imageWindowKey is the preferences key for the geometry of the image window of
// the i-th aperture.
func imageWindowKey(i int) string { return fmt.Sprintf("imageWindow%d", i+1) }

// openImageWindow opens the image window for curves[i]. Closing that window
// deselects the aperture, so a window is open exactly while its aperture is selected.
func (v *viewer) openImageWindow(i int) {
	if v.imageWins[i] != nil {
		return
	}
	iw := newImageWindow(v.app, v.curves[i])
	iw.red = v.redLevel
	v.imageWins[i] = iw
	restoreWindowSize(v.prefs, iw.win, imageWindowKey(i), fyne.NewSize(640, 420))
	iw.win.SetCloseIntercept(func() { v.checks[i].SetChecked(false) })
	if len(v.frames) > 0 {
		iw.showFrame(v.frames[v.cursor], v.useAppsum)
	}
	iw.win.Show()
	restoreWindowPosition(v.prefs, iw.win, imageWindowKey(i))
}

// closeImageWindow saves the geometry of curves[i]'s image window and closes it.
func (v *viewer) closeImageWindow(i int) {
	if iw := v.imageWins[i]; iw != nil {
		saveWindowGeometry(v.prefs, iw.win, imageWindowKey(i))
		iw.win.Close()
		v.imageWins[i] = nil
	}
}

// prefFrameWindow is the preferences key for the initial frame window's geometry.
const prefFrameWindow = "frameWindow"

// openFrameWindow shows the initial frame window, or brings it to the front if
// it is already open.
func (v *viewer) openFrameWindow() {
	if v.frameWin != nil {
		v.frameWin.RequestFocus()
		return
	}
	v.frameWin = newFrameWindow(v.app, v.header, "Initial frame - "+filepath.Base(v.path))
	// Big enough for the frame at full size plus the padding, the size line and
	// the aperture table (a heading and a row per aperture), within reason.
	def := fyne.NewSize(
		float32(min(max(v.header.FrameWidth+60, 640), maxFrameWindowWidth)),
		float32(min(v.header.FrameHeight+110+40*(min(len(v.header.Apertures), maxVisibleApertures)+1), maxFrameWindowHeight)),
	)
	restoreWindowSize(v.prefs, v.frameWin, prefFrameWindow, def)
	v.frameWin.SetCloseIntercept(v.closeFrameWindow)
	v.frameWin.Show()
	restoreWindowPosition(v.prefs, v.frameWin, prefFrameWindow)
}

// closeFrameWindow saves the initial frame window's geometry and closes it.
func (v *viewer) closeFrameWindow() {
	if v.frameWin != nil {
		saveWindowGeometry(v.prefs, v.frameWin, prefFrameWindow)
		v.frameWin.Close()
		v.frameWin = nil
	}
}

// closeWindows closes every image window and the initial frame window, as when
// another file is opened or the app closes.
func (v *viewer) closeWindows() {
	for i := range v.imageWins {
		v.closeImageWindow(i)
	}
	v.closeFrameWindow()
}

// restoreWindowPositions moves the open image windows and the initial frame
// window to their saved positions. Windows opened before the app started
// running have no native window until then, so this is called again once it has.
func (v *viewer) restoreWindowPositions() {
	for i, iw := range v.imageWins {
		if iw != nil {
			restoreWindowPosition(v.prefs, iw.win, imageWindowKey(i))
		}
	}
	if v.frameWin != nil {
		restoreWindowPosition(v.prefs, v.frameWin, prefFrameWindow)
	}
}

// resetLimits sets the display limits to their defaults for the selected curves.
func (v *viewer) resetLimits() {
	begin, end := v.frameRange()
	lo, hi := defaultYLimits(v.settings().Curves, v.useAppsum, begin, end)
	v.yMinEntry.SetText(formatLimit(lo)) // the entries' OnChanged set yMin and yMax
	v.yMaxEntry.SetText(formatLimit(hi))
	v.redraw()
}

// apertureRows returns a row per aperture: a check box to show its light curve,
// a swatch of its color, and a button to change the color.
func (v *viewer) apertureRows() fyne.CanvasObject {
	rows := container.NewVBox()
	for i, c := range v.curves {
		check := widget.NewCheck(c.Name, nil)
		check.SetChecked(v.selected[i]) // before OnChanged: the limit entries don't exist yet
		check.OnChanged = func(on bool) {
			v.selected[i] = on
			v.resetLimits()
			v.setCursor(v.cursor)
			if on {
				v.openImageWindow(i)
			} else {
				v.closeImageWindow(i)
			}
		}
		v.checks = append(v.checks, check)

		swatch := canvas.NewRectangle(v.colors[i])
		swatch.SetMinSize(fyne.NewSize(24, 16))
		colorButton := widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
			picker := dialog.NewColorPicker("Curve color", "Color for "+c.Name, func(col color.Color) {
				v.colors[i] = col
				swatch.FillColor = col
				swatch.Refresh()
				v.redraw()
			}, v.win)
			picker.Advanced = true
			picker.SetColor(v.colors[i])
			picker.Show()
		})

		rows.Add(container.NewBorder(nil, nil, nil,
			container.NewHBox(container.NewCenter(swatch), colorButton), check))
	}
	return scrollIfMany(rows, len(v.curves))
}

// fieldCheck returns the check box that switches the plot from intensity to appsum.
func (v *viewer) fieldCheck() fyne.CanvasObject {
	return widget.NewCheck("Plot appsum instead of intensity", func(on bool) {
		v.useAppsum = on
		v.resetLimits()
		v.setCursor(v.cursor)
	})
}

// frameRange returns the begin and end frames shown: those entered, or every
// frame if the begin frame is not before the end frame.
func (v *viewer) frameRange() (begin, end float64) {
	if v.xMin < v.xMax {
		return v.xMin, v.xMax
	}
	return v.allXMin, v.allXMax
}

// visibleFrames returns the indexes in frames of the first and last frame
// shown, which the cursor stays between; every frame if none is shown.
func (v *viewer) visibleFrames() (first, last int) {
	begin, end := v.frameRange()
	first = sort.SearchFloat64s(v.frames, begin)
	last = sort.SearchFloat64s(v.frames, end)
	if last == len(v.frames) || v.frames[last] > end {
		last--
	}
	if first > last {
		return 0, len(v.frames) - 1
	}
	return first, last
}

// frameRangeControls returns the entries for the begin and end frames shown,
// and a button that shows every frame again. Frames are applied as they are
// typed; a begin frame not before the end frame shows every frame.
func (v *viewer) frameRangeControls() fyne.CanvasObject {
	begin := numberEntry(func(x float64) { v.xMin = x; v.frameRangeChanged() })
	end := numberEntry(func(x float64) { v.xMax = x; v.frameRangeChanged() })
	showAll := func() {
		begin.SetText(fmt.Sprint(v.allXMin))
		end.SetText(fmt.Sprint(v.allXMax))
	}
	showAll()
	return container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Begin frame", begin),
			widget.NewFormItem("End frame", end),
		),
		widget.NewButton("All frames", showAll),
	)
}

// frameRangeChanged keeps the cursor within the frames shown and redraws.
// It leaves the display limits alone; "Default limits" fits them to the frames shown.
func (v *viewer) frameRangeChanged() { v.setCursor(v.cursor) }

// numberEntry returns an entry that calls set with its value whenever it is
// changed to a valid number, and marks it invalid otherwise.
func numberEntry(set func(float64)) *widget.Entry {
	e := widget.NewEntry()
	e.Validator = func(s string) error {
		_, err := strconv.ParseFloat(s, 64)
		return err
	}
	e.OnChanged = func(s string) {
		if x, err := strconv.ParseFloat(s, 64); err == nil {
			set(x)
		}
	}
	return e
}

// limitControls returns the entries for the lower and upper display limits and
// a button that restores their defaults. Limits are applied as they are typed;
// a pair with the lower limit not below the upper is ignored.
func (v *viewer) limitControls() fyne.CanvasObject {
	v.yMinEntry = numberEntry(func(x float64) { v.yMin = x; v.redraw() })
	v.yMaxEntry = numberEntry(func(x float64) { v.yMax = x; v.redraw() })

	return container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Upper limit", v.yMaxEntry),
			widget.NewFormItem("Lower limit", v.yMinEntry),
		),
		widget.NewButton("Default limits", v.resetLimits),
	)
}

// dotSizeControl returns the slider for the dot size, which is kept in the preferences.
func (v *viewer) dotSizeControl() fyne.CanvasObject {
	label := widget.NewLabel("")
	showSize := func() { label.SetText(fmt.Sprintf("Dot size: %g", v.dotSize)) }
	showSize()

	slider := widget.NewSlider(1, 12)
	slider.Step = 0.5
	slider.SetValue(v.dotSize)
	slider.OnChanged = func(x float64) {
		v.dotSize = x
		v.prefs.SetFloat(prefDotSize, x)
		showSize()
		v.redraw()
	}
	return container.NewVBox(label, slider)
}

// redLevelControl returns the entry for the pixel value at or above which the
// aperture images show red. It starts at the records' saturation value; blank
// shows no red pixels.
func (v *viewer) redLevelControl() fyne.CanvasObject {
	e := widget.NewEntry()
	e.SetPlaceHolder("blank: none")
	e.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		_, err := strconv.ParseFloat(s, 64)
		return err
	}
	e.OnChanged = func(s string) {
		level := math.Inf(1)
		if strings.TrimSpace(s) != "" {
			x, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return
			}
			level = x
		}
		v.redLevel = level
		for _, iw := range v.imageWins {
			if iw != nil {
				iw.setRedLevel(level)
			}
		}
	}
	if v.saturation > 0 {
		e.SetText(fmt.Sprint(v.saturation))
	}
	return widget.NewForm(widget.NewFormItem("Red pixels at or above", e))
}

// formatLimit formats a display limit for its entry, to 2 decimal places at most.
func formatLimit(x float64) string {
	return strconv.FormatFloat(math.Round(x*100)/100, 'f', -1, 64)
}

func boldLabel(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

// truncatedLabel returns a label that shows "…" rather than widening its
// container for long text such as a full path.
func truncatedLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// maxVisibleApertures is how many aperture rows are shown at once; more scroll.
const maxVisibleApertures = 6

// scrollIfMany returns rows, a container with a row (or, for a grid, a cell)
// per aperture in its first objects, as they are when there are at most
// maxVisibleApertures, and otherwise in a scroll box tall enough for that many.
func scrollIfMany(rows *fyne.Container, count int) fyne.CanvasObject {
	if count <= maxVisibleApertures || len(rows.Objects) == 0 {
		return rows
	}
	rowHeight := rows.Objects[0].MinSize().Height
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(0, maxVisibleApertures*rowHeight+(maxVisibleApertures-1)*theme.Padding()))
	return scroll
}
