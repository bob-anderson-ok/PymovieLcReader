package main

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestScaledImage(t *testing.T) {
	// 2×2: pixel (row, col) is at row*2+col.
	pixels := []uint16{100, 150, 200, 50}
	img := scaledImage(pixels, nil, 2, 100, 200)
	gray := func(x, y int) uint8 { return img.NRGBAAt(x, y).R }
	if gray(0, 0) != 0 || gray(1, 0) != 128 || gray(0, 1) != 255 || gray(1, 1) != 0 {
		t.Errorf("grays = %d %d %d %d", gray(0, 0), gray(1, 0), gray(0, 1), gray(1, 1))
	}

	masked := scaledImage(pixels, []uint8{0, 1, 1, 0}, 2, 100, 200)
	if masked.NRGBAAt(0, 0) != maskClosedColor || masked.NRGBAAt(1, 1) != maskClosedColor {
		t.Error("closed mask pixels should have maskClosedColor")
	}
	if got := masked.NRGBAAt(1, 0); got != (color.NRGBA{R: 128, G: 128, B: 128, A: 255}) {
		t.Errorf("open mask pixel = %v, want the image data", got)
	}

	// Crossed levels threshold at black rather than dividing by zero.
	if th := scaledImage(pixels, nil, 2, 120, 120); th.NRGBAAt(0, 0).R != 0 || th.NRGBAAt(1, 0).R != 255 {
		t.Error("threshold with equal levels is wrong")
	}
}

// TestImageWindowsFollowSelection checks an image window is open exactly while
// its aperture is selected, and shows the cursor frame.
func TestImageWindowsFollowSelection(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, v := newViewer("LC-test.pymovie", a, w)
	w.SetContent(content)

	if v.imageWins[0] == nil || v.imageWins[1] != nil {
		t.Fatal("only the first aperture's window should start open")
	}
	test.Tap(v.checks[1])
	if v.imageWins[1] == nil {
		t.Fatal("selecting an aperture should open its window")
	}
	v.setCursor(7)
	if iw := v.imageWins[1]; iw.point != 7 || iw.info.Text[:7] != "Frame 7" {
		t.Errorf("image window shows point %d, %q", iw.point, iw.info.Text)
	}
	if want := fmt.Sprintf("Intensity: %.6g", v.curves[1].Intensity[7]); !strings.Contains(v.imageWins[1].info.Text, want) {
		t.Errorf("info = %q, want it to contain %q", v.imageWins[1].info.Text, want)
	}
	v.useAppsum = true
	v.setCursor(7)
	if want := fmt.Sprintf("Appsum: %.6g", v.curves[1].Appsum[7]); !strings.Contains(v.imageWins[1].info.Text, want) {
		t.Errorf("info = %q, want it to contain %q", v.imageWins[1].info.Text, want)
	}

	test.Tap(v.checks[1])
	if v.imageWins[1] != nil {
		t.Error("deselecting an aperture should close its window")
	}

	v.checks[2].SetChecked(true)
	v.closeImageWindows()
	for i, iw := range v.imageWins {
		if iw != nil {
			t.Errorf("window %d still open", i)
		}
	}
}
