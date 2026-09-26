package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

func TestBuildLightCurvesSortsByFrame(t *testing.T) {
	recs := []pymoviefile.Record{
		{Name: "target", Frame: 2, Intensity: 20, Timestamp: "00:00:02"},
		{Name: "comp", Frame: 2, Intensity: 200, Timestamp: "00:00:02"},
		{Name: "target", Frame: 1, Intensity: 10, Timestamp: "00:00:01"},
		{Name: "comp", Frame: 1, Intensity: 100, Timestamp: "00:00:01"},
	}
	curves := buildLightCurves(recs)
	if len(curves) != 2 || curves[0].Name != "target" || curves[1].Name != "comp" {
		t.Fatalf("curves = %+v", curves)
	}
	if c := curves[0]; c.Frames[0] != 1 || c.Intensity[0] != 10 || c.Timestamps[1] != "00:00:02" {
		t.Errorf("target = %+v", c)
	}

	tk := newTimeTicker(curves)
	if tk == nil || tk.stampAt(1.4) != "00:00:01" || tk.stampAt(1.6) != "00:00:02" || tk.stampAt(9) != "00:00:02" {
		t.Errorf("ticker = %+v", tk)
	}
	recs[3].Timestamp = ""
	if newTimeTicker(buildLightCurves(recs)) != nil {
		t.Error("a blank timestamp should give frame numbers")
	}

	if lo, hi := defaultYLimits(curves[:1], false, 0, 9); lo != 0 || hi != 22 {
		t.Errorf("limits = %v, %v; want 0, 22", lo, hi)
	}

	if lo, hi := defaultYLimits(curves[:1], false, 1, 1); lo != 0 || hi != 11 {
		t.Errorf("limits for frame 1 only = %v, %v; want 0, 11", lo, hi)
	}
	neg := []*lightCurve{{Frames: []float64{0, 1, 2}, Intensity: []float64{-10, 5, 20}}}
	if lo, hi := defaultYLimits(neg, false, 0, 2); lo != -11 || hi != 22 {
		t.Errorf("limits with a negative value = %v, %v; want -11, 22", lo, hi)
	}
	allNeg := []*lightCurve{{Frames: []float64{0, 1}, Intensity: []float64{-10, -5}}}
	if lo, hi := defaultYLimits(allNeg, false, 0, 1); lo != -11 || hi != 0 {
		t.Errorf("limits with only negative values = %v, %v; want -11, 0", lo, hi)
	}
}

func TestSaturationLevel(t *testing.T) {
	recs := []pymoviefile.Record{{Saturation: 0}, {Saturation: 900}, {Saturation: 800}, {Saturation: 900}}
	if got := saturationLevel(recs); got != 900 {
		t.Errorf("saturation = %d, want the most common, 900", got)
	}
	if got := saturationLevel(recs[:1]); got != 0 {
		t.Errorf("saturation = %d, want 0 when not set", got)
	}
	if got := saturationLevel(recs[1:3]); got != 800 {
		t.Errorf("saturation = %d, want the smaller on a tie, 800", got)
	}
}

// TestViewerOnSampleFile builds the viewer for LC-test.pymovie in a test window,
// works its controls, and renders the plot.
func TestViewerOnSampleFile(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, _ := newViewer("LC-test.pymovie", a, w)
	w.SetContent(content)
	w.Resize(fyne.NewSize(1000, 600))

	split := w.Content().(*container.Split)
	var checks []*widget.Check
	var limits []*widget.Entry
	collect(split.Leading, &checks, &limits)
	if len(checks) != 5 || len(limits) != 5 { // 4 apertures + appsum; begin, end, upper, lower, red
		t.Fatalf("found %d checks, %d entries", len(checks), len(limits))
	}
	upper := limits[2].Text
	test.Tap(checks[1]) // add tracker
	test.Tap(checks[4]) // appsum
	if limits[2].Text == upper {
		t.Error("upper limit did not change with the selection")
	}
	if w.Canvas().Capture() == nil { // renders the plot, which the tap below needs
		t.Fatal("no image")
	}

	// The cursor starts on the first frame and moves with the arrow keys.
	pw := split.Trailing.(*fyne.Container).Objects[0].(*plotWidget)
	readout := split.Trailing.(*fyne.Container).Objects[1].(*widget.Label)
	if !strings.HasPrefix(readout.Text, "Frame 0 ") {
		t.Errorf("readout = %q", readout.Text)
	}
	test.Tap(pw) // focuses the plot
	pw.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	for range 3 {
		w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	w.Canvas().Unfocus() // with nothing focused, the window's key handler moves it
	w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyLeft})

	// Tapping the middle of the x axis moves the cursor to the middle frame.
	mid := (pw.axis.Left + pw.axis.Right) / 2 * float64(pw.Size().Width) / float64(pw.rasterW)
	test.TapAt(pw, fyne.NewPos(float32(mid), 50))
	if !strings.HasPrefix(readout.Text, "Frame 175 ") {
		t.Errorf("after tapping the middle, readout = %q", readout.Text)
	}
}

// TestFrameRange checks the begin and end frame entries set the frames shown
// and keep the cursor within them.
func TestFrameRange(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, v := newViewer("LC-test.pymovie", a, w)
	w.SetContent(content)
	var checks []*widget.Check
	var entries []*widget.Entry
	collect(content.(*container.Split).Leading, &checks, &entries)
	begin, end := entries[0], entries[1]
	if begin.Text != "0" || end.Text != "350" {
		t.Fatalf("begin, end = %q, %q", begin.Text, end.Text)
	}

	begin.SetText("100")
	end.SetText("200")
	if s := v.settings(); s.XMin != 100 || s.XMax != 200 || s.Cursor != 100 {
		t.Errorf("x range %v-%v, cursor %v; want 100-200, cursor 100", s.XMin, s.XMax, s.Cursor)
	}
	v.setCursor(1000)
	if s := v.settings(); s.Cursor != 200 {
		t.Errorf("cursor = %v, want it kept at the end frame 200", s.Cursor)
	}

	begin.SetText("300") // after the end frame: every frame is shown
	if s := v.settings(); s.XMin != 0 || s.XMax != 350 {
		t.Errorf("x range %v-%v, want every frame", s.XMin, s.XMax)
	}
}

// TestCursorKeys checks the cursor steps and stays within the frames.
func TestCursorKeys(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, _ := newViewer("LC-test.pymovie", a, w)
	w.SetContent(content)
	key := w.Canvas().OnTypedKey()
	pw := findPlot(w.Content())
	key(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if s := pw.settings(); s.Cursor != 0 {
		t.Errorf("cursor = %v after Left at the start", s.Cursor)
	}
	for range 5 {
		key(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	if s := pw.settings(); s.Cursor != 5 || !s.ShowCursor {
		t.Errorf("cursor = %v after 5 Rights", s.Cursor)
	}
}

func findPlot(o fyne.CanvasObject) *plotWidget {
	switch x := o.(type) {
	case *plotWidget:
		return x
	case *container.Split:
		if p := findPlot(x.Leading); p != nil {
			return p
		}
		return findPlot(x.Trailing)
	case *fyne.Container:
		for _, c := range x.Objects {
			if p := findPlot(c); p != nil {
				return p
			}
		}
	}
	return nil
}

func collect(o fyne.CanvasObject, checks *[]*widget.Check, entries *[]*widget.Entry) {
	switch x := o.(type) {
	case *widget.Check:
		*checks = append(*checks, x)
	case *widget.Entry:
		*entries = append(*entries, x)
	case *container.Scroll:
		collect(x.Content, checks, entries)
	case *widget.Form:
		for _, it := range x.Items {
			collect(it.Widget, checks, entries)
		}
	case *fyne.Container:
		for _, c := range x.Objects {
			collect(c, checks, entries)
		}
	}
}
