package main

import (
	"image"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// pixelView shows an n×n aperture image with square pixels, scaled to fit, and
// reports which pixel the mouse pointer is over.
type pixelView struct {
	widget.BaseWidget
	img *canvas.Image
	n   int // pixels across the image; 0 while there is none

	onHover func(row, col int, ok bool) // ok is false when the pointer is over no pixel
}

func newPixelView() *pixelView {
	img := canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillContain
	img.ScaleMode = canvas.ImageScalePixels
	img.SetMinSize(fyne.NewSize(160, 160))
	p := &pixelView{img: img}
	p.ExtendBaseWidget(p)
	return p
}

// setImage shows img, an n×n image; n is 0 for no image.
func (p *pixelView) setImage(img image.Image, n int) {
	p.img.Image = img
	p.n = n
	p.img.Refresh()
}

func (p *pixelView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.img)
}

func (p *pixelView) MouseIn(e *desktop.MouseEvent)    { p.hover(e.Position) }
func (p *pixelView) MouseMoved(e *desktop.MouseEvent) { p.hover(e.Position) }
func (p *pixelView) MouseOut() {
	if p.onHover != nil {
		p.onHover(0, 0, false)
	}
}

func (p *pixelView) hover(pos fyne.Position) {
	if p.onHover != nil {
		row, col, ok := pixelAt(pos, p.Size(), p.n)
		p.onHover(row, col, ok)
	}
}

// pixelAt returns the row and column of the pixel at pos in an n×n image drawn
// to fit, centred, in an area of the given size (canvas.ImageFillContain).
// ok is false if pos is outside the image or there is no image.
func pixelAt(pos fyne.Position, size fyne.Size, n int) (row, col int, ok bool) {
	side := min(size.Width, size.Height) // the image is square
	if n <= 0 || side <= 0 {
		return 0, 0, false
	}
	x := float64(pos.X - (size.Width-side)/2)
	y := float64(pos.Y - (size.Height-side)/2)
	cell := float64(side) / float64(n)
	col, row = int(math.Floor(x/cell)), int(math.Floor(y/cell))
	if col < 0 || row < 0 || col >= n || row >= n {
		return 0, 0, false
	}
	return row, col, true
}
