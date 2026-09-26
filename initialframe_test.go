package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

// go-reader/testdata/sample.pymovie has a 40 x 30 initial frame with a green box
// for 'target ñ' and a yellow one for 'comp'.
const frameSample = "go-reader/testdata/sample.pymovie"

func TestFrameImage(t *testing.T) {
	h, _, err := pymoviefile.ReadFile(frameSample)
	if err != nil {
		t.Fatal(err)
	}
	img := frameImage(h)
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 30 {
		t.Fatalf("image is %v", b)
	}
	if got := img.NRGBAAt(5, 4); got != (color.NRGBA{G: 255, A: 255}) { // x=5, y=4: the green box corner
		t.Errorf("box corner = %v, want green", got)
	}
	if frameSizeText(h) != "40 × 30 pixels" || !hasInitialFrame(h) {
		t.Errorf("frame size text = %q", frameSizeText(h))
	}
	if centroidText(h.Apertures[0]) != "(15.50, 14.25)" || centroidText(h.Apertures[1]) != "not known" {
		t.Errorf("centroids = %q, %q", centroidText(h.Apertures[0]), centroidText(h.Apertures[1]))
	}
}

func TestFrameWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, v := newViewer(frameSample, a, w)
	w.SetContent(content)

	if v.frameWin == nil {
		t.Fatal("the initial frame window should open with the file")
	}
	if w := v.frameWin; w.Title() != "Initial frame - sample.pymovie" {
		t.Errorf("title = %q", w.Title())
	}
	v.closeWindows()
	if v.frameWin != nil {
		t.Error("closeWindows left the initial frame window open")
	}

	// LC-test.pymovie predates the initial frame section: its button is disabled.
	content, v = newViewer("LC-test.pymovie", a, w)
	if b := findButton(content, "Show initial frame"); b == nil || !b.Disabled() {
		t.Errorf("Show initial frame button = %v, want disabled", b)
	}
	if v.frameWin != nil {
		t.Error("a file with no initial frame should not open its window")
	}
}

// findButton returns the button with the given text in the viewer content, or nil.
func findButton(o fyne.CanvasObject, text string) *widget.Button {
	switch x := o.(type) {
	case *widget.Button:
		if x.Text == text {
			return x
		}
	case *container.Split:
		if b := findButton(x.Leading, text); b != nil {
			return b
		}
		return findButton(x.Trailing, text)
	case *container.Scroll:
		return findButton(x.Content, text)
	case *fyne.Container:
		for _, c := range x.Objects {
			if b := findButton(c, text); b != nil {
				return b
			}
		}
	}
	return nil
}
