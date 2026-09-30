package main

import (
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"pymoviefile"
)

// newSampleFrameView returns a frame view of frameSample's 40 × 30 frame, 400 ×
// 300 in size, so it fits at 10 units a pixel with no border, and the text its
// pixel info would show.
func newSampleFrameView(t *testing.T) (*frameView, *string) {
	t.Helper()
	h, _, err := pymoviefile.ReadFile(frameSample)
	if err != nil {
		t.Fatal(err)
	}
	f := newFrameView(frameImage(h), h.Apertures)
	info := new(string)
	f.onHover = func(row, col int, ok bool) { *info = framePixelText(h, row, col, ok) }
	test.WidgetRenderer(f) // lay the view out as it resizes
	f.Resize(fyne.NewSize(400, 300))
	return f, info
}

func moveTo(f *frameView, x, y float32) {
	f.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}})
}

func TestFrameViewHover(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	f, info := newSampleFrameView(t)
	if f.scale != 10 || f.off != (fyne.Position{}) {
		t.Fatalf("fitted at scale %v, offset %v", f.scale, f.off)
	}

	moveTo(f, 5, 5) // pixel (0, 0): gray, in no aperture
	if *info != "Pixel x 0, y 0:  0" || f.balloon.Visible() {
		t.Errorf("at (0, 0): %q, balloon shown %v", *info, f.balloon.Visible())
	}
	moveTo(f, 55, 45) // the green box's corner, x 5, y 4
	if *info != "Pixel x 5, y 4:  R 0, G 255, B 0" {
		t.Errorf("on the box corner: %q", *info)
	}
	if !f.balloon.Visible() || f.balloonText.Text != "target ñ" || f.balloon.StrokeColor != apertureBoxColor("green") {
		t.Errorf("on the box corner, balloon shown %v with %q", f.balloon.Visible(), f.balloonText.Text)
	}
	moveTo(f, 155, 145) // inside the box, off its outline
	if !f.balloon.Visible() || f.balloonText.Text != "target ñ" {
		t.Errorf("inside the box, balloon shown %v with %q", f.balloon.Visible(), f.balloonText.Text)
	}
	moveTo(f, 395, 85) // the comp box's top right corner: the balloon stays in view
	if f.balloonText.Text != "comp" || f.balloon.Position().X+f.balloon.Size().Width > 400 {
		t.Errorf("at the right edge, balloon %q at %v", f.balloonText.Text, f.balloon.Position())
	}
	moveTo(f, 265, 145) // between the boxes
	if f.balloon.Visible() {
		t.Error("between the boxes, the balloon should be hidden")
	}
	f.MouseOut()
	if *info != frameInfoPrompt || f.balloon.Visible() {
		t.Errorf("after leaving: %q, balloon shown %v", *info, f.balloon.Visible())
	}
}

func TestFrameViewZoomAndDrag(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	f, info := newSampleFrameView(t)

	// A wheel notch up zooms in about the pointer, which stays over the same pixel.
	pos := fyne.NewPos(123, 87)
	f.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: pos}, Scrolled: fyne.Delta{DY: scrollNotch}})
	if f.scale != 10*zoomPerScroll || f.fitted {
		t.Errorf("zoomed in to scale %v", f.scale)
	}
	if row, col, _ := f.frameAt(pos); row != 8 || col != 12 {
		t.Errorf("after zooming, the pointer is over x %d, y %d; want x 12, y 8", col, row)
	}
	if *info != "Pixel x 12, y 8:  78" {
		t.Errorf("after zooming: %q", *info)
	}

	// Zooming out stops at a quarter of the fitted size.
	for range 20 {
		f.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: pos}, Scrolled: fyne.Delta{DY: -scrollNotch}})
	}
	if f.scale != 2.5 {
		t.Errorf("zoomed out to scale %v, want 2.5", f.scale)
	}

	// Dragging moves the frame, but not out of view.
	f.DoubleTapped(nil)
	f.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: pos}, Dragged: fyne.Delta{DX: 20, DY: -10}})
	if f.off != fyne.NewPos(20, -10) {
		t.Errorf("dragged to %v, want (20, -10)", f.off)
	}
	f.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: pos}, Dragged: fyne.Delta{DX: 1000}})
	if f.off.X != 400-minDragMargin {
		t.Errorf("dragged off the right to x %v, want %v", f.off.X, 400-minDragMargin)
	}

	// A double click or a right click fits the frame again.
	for name, fit := range map[string]func(){
		"double click": func() { f.DoubleTapped(nil) },
		"right click":  func() { f.TappedSecondary(nil) },
	} {
		f.zoom(pos, 2)
		fit()
		if f.scale != 10 || f.off != (fyne.Position{}) || !f.fitted {
			t.Errorf("after a %s, scale %v, offset %v", name, f.scale, f.off)
		}
	}
}

// TestFrameWindowEscape checks that Escape in the initial frame window fits a
// zoomed frame to the view again.
func TestFrameWindowEscape(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	h, _, err := pymoviefile.ReadFile(frameSample)
	if err != nil {
		t.Fatal(err)
	}
	w := newFrameWindow(a, h, "frame")
	w.Resize(fyne.NewSize(600, 500))
	f := findFrameView(w.Content())
	if f == nil {
		t.Fatal("no frame view in the window")
	}
	fitted := f.scale
	f.zoom(fyne.NewPos(50, 50), 3)
	w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if f.scale != fitted || !f.fitted {
		t.Errorf("after Escape, scale %v, want %v", f.scale, fitted)
	}
}

func findFrameView(o fyne.CanvasObject) *frameView {
	switch x := o.(type) {
	case *frameView:
		return x
	case *fyne.Container:
		for _, c := range x.Objects {
			if f := findFrameView(c); f != nil {
				return f
			}
		}
	}
	return nil
}

func TestFrameViewRender(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	f, _ := newSampleFrameView(t)
	green := color.NRGBA{G: 255, A: 255}

	img := f.render(800, 600).(*image.NRGBA) // 2 output pixels a unit: 20 a frame pixel
	if got := img.NRGBAAt(5*20+10, 4*20+10); got != green {
		t.Errorf("zoomed in, the box corner is %v", got)
	}

	// Zoomed out to 2.5 units a frame pixel, at a quarter of an output pixel a
	// unit, each output pixel covers parts of up to 3 × 3 frame pixels, and keeps
	// the box outlines.
	f.zoom(fyne.NewPos(0, 0), 0.25)
	img = f.render(100, 75).(*image.NRGBA)
	if got := img.NRGBAAt(3, 6); got != green { // frame x 4.8 to 6.4, y 9.6 to 11.2
		t.Errorf("zoomed out, the box's left edge is %v", got)
	}
	if got := img.NRGBAAt(99, 74); got.A != 0 {
		t.Errorf("off the frame: %v, want transparent", got)
	}
}

func TestBlockColor(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 10, B: 10, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
	src.SetNRGBA(0, 1, color.NRGBA{R: 30, G: 30, B: 30, A: 255})
	src.SetNRGBA(1, 1, color.NRGBA{R: 40, G: 40, B: 40, A: 255})
	if got := blockColor(src, 0, 2, 0, 2); got != (color.NRGBA{R: 25, G: 25, B: 25, A: 255}) {
		t.Errorf("gray block = %v, want their average", got)
	}
	src.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	if got := blockColor(src, 0, 2, 0, 2); got != (color.NRGBA{R: 255, A: 255}) {
		t.Errorf("block with a red pixel = %v, want red", got)
	}
}
