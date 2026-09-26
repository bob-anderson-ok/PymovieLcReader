package main

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestPixelAt(t *testing.T) {
	// A 10×10 image in a 200 wide, 100 high area: 100 pixels square, centred,
	// so it starts at x = 50 and each pixel is 10 across.
	size := fyne.NewSize(200, 100)
	for _, c := range []struct {
		x, y     float32
		row, col int
		ok       bool
	}{
		{50, 0, 0, 0, true},
		{59.9, 9.9, 0, 0, true},
		{60, 10, 1, 1, true},
		{149.9, 99.9, 9, 9, true},
		{49.9, 50, 0, 0, false}, // left of the image
		{150, 50, 0, 0, false},  // right of it
	} {
		row, col, ok := pixelAt(fyne.NewPos(c.x, c.y), size, 10)
		if ok != c.ok || (ok && (row != c.row || col != c.col)) {
			t.Errorf("pixelAt(%v, %v) = %d, %d, %v; want %d, %d, %v", c.x, c.y, row, col, ok, c.row, c.col, c.ok)
		}
	}
	if _, _, ok := pixelAt(fyne.NewPos(1, 1), size, 0); ok {
		t.Error("no image should have no pixels")
	}
}

// TestPixelHover moves the pointer over an aperture image and checks the value
// shown, including after the cursor moves to another frame.
func TestPixelHover(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	content, v := newViewer("LC-test.pymovie", a, w)
	w.SetContent(content)
	iw := v.imageWins[0]
	iw.win.Resize(fyne.NewSize(640, 420))

	if iw.pixelInfo.Text != pixelInfoPrompt {
		t.Errorf("before hovering: %q", iw.pixelInfo.Text)
	}
	// The centre of the raw image is pixel (25, 25) of the 51×51 aperture.
	size := iw.raw.Size()
	iw.raw.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(size.Width/2, size.Height/2)}})
	c := v.curves[0]
	want := fmt.Sprintf("Pixel x 25, y 25:  %d", c.Images[0][25*51+25])
	if !strings.HasPrefix(iw.pixelInfo.Text, want) || !strings.Contains(iw.pixelInfo.Text, "sampling mask") {
		t.Errorf("hovering: %q, want it to start %q", iw.pixelInfo.Text, want)
	}

	v.setCursor(10)
	want = fmt.Sprintf("Pixel x 25, y 25:  %d", c.Images[10][25*51+25])
	if !strings.HasPrefix(iw.pixelInfo.Text, want) {
		t.Errorf("after moving the cursor: %q, want it to start %q", iw.pixelInfo.Text, want)
	}

	iw.raw.MouseOut()
	if iw.pixelInfo.Text != pixelInfoPrompt {
		t.Errorf("after leaving: %q", iw.pixelInfo.Text)
	}
}

func TestScrollIfMany(t *testing.T) {
	rows := func(n int) *fyne.Container {
		c := container.NewVBox()
		for range n {
			c.Add(widget.NewLabel("aperture"))
		}
		return c
	}
	if _, ok := scrollIfMany(rows(6), 6).(*fyne.Container); !ok {
		t.Error("6 apertures should not scroll")
	}
	r := rows(9)
	s, ok := scrollIfMany(r, 9).(*container.Scroll)
	if !ok {
		t.Fatal("9 apertures should scroll")
	}
	rowHeight := r.Objects[0].MinSize().Height
	if h := s.MinSize().Height; h < 6*rowHeight || h >= 7*rowHeight {
		t.Errorf("scroll box is %v high, want room for 6 rows of %v", h, rowHeight)
	}
}
