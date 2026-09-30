package main

import (
	"image"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

// frameView shows the initial frame, which the mouse can zoom (scroll wheel,
// about the pointer) and move (drag); a double click, a right click or the
// Escape key fits it to the view again. It reports the frame pixel under the pointer, and while the pointer is
// in or on an aperture box, shows the aperture's name in a balloon beside it.
type frameView struct {
	widget.BaseWidget
	img       *image.NRGBA
	apertures []pymoviefile.Aperture

	raster      *canvas.Raster
	balloon     *canvas.Rectangle
	balloonText *canvas.Text

	scale  float64       // view units per frame pixel
	off    fyne.Position // where the top-left corner of frame pixel (0, 0) is drawn
	fitted bool          // the frame is fitted to the view, and stays fitted as it resizes

	pointer   fyne.Position // the pointer's last position in the view
	pointerIn bool

	onHover func(row, col int, ok bool) // ok is false when the pointer is over no pixel
}

const (
	zoomPerScroll = 1.2 // how much a scroll wheel notch zooms
	scrollNotch   = 25  // the scroll distance Fyne gives a wheel notch on Windows and Linux
	maxFrameScale = 64  // the most view units a frame pixel may cover
	minDragMargin = 32  // how much of the frame, in view units, a drag must leave in view
	balloonGap    = 14  // how far the balloon sits from the pointer
	balloonPad    = 4   // the space around the balloon's text
)

var balloonColor = color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xe8}

func newFrameView(img *image.NRGBA, apertures []pymoviefile.Aperture) *frameView {
	f := &frameView{img: img, apertures: apertures, fitted: true}
	f.raster = canvas.NewRaster(f.render)
	f.balloon = canvas.NewRectangle(balloonColor)
	f.balloon.StrokeWidth = 1
	f.balloon.CornerRadius = 4
	f.balloonText = canvas.NewText("", color.White)
	f.balloon.Hide()
	f.balloonText.Hide()
	f.ExtendBaseWidget(f)
	return f
}

func (f *frameView) CreateRenderer() fyne.WidgetRenderer {
	return &frameViewRenderer{f: f}
}

type frameViewRenderer struct{ f *frameView }

func (r *frameViewRenderer) Layout(size fyne.Size) {
	f := r.f
	f.raster.Resize(size)
	if f.fitted {
		f.fit(size)
	} else {
		f.clampOffset(size)
	}
	f.layoutBalloon()
}

func (r *frameViewRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 150) }
func (r *frameViewRenderer) Refresh() {
	r.Layout(r.f.Size())
	r.f.raster.Refresh()
	r.f.balloon.Refresh()
	r.f.balloonText.Refresh()
}
func (r *frameViewRenderer) Destroy() {}
func (r *frameViewRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.f.raster, r.f.balloon, r.f.balloonText}
}

// fit scales the frame to fill a view of the given size, centred.
func (f *frameView) fit(size fyne.Size) {
	w, h := f.img.Bounds().Dx(), f.img.Bounds().Dy()
	if w == 0 || h == 0 || size.Width <= 0 || size.Height <= 0 {
		return
	}
	f.scale = min(float64(size.Width)/float64(w), float64(size.Height)/float64(h))
	f.off = fyne.NewPos(
		(size.Width-float32(float64(w)*f.scale))/2,
		(size.Height-float32(float64(h)*f.scale))/2,
	)
}

// clampOffset keeps some of the frame in a view of the given size.
func (f *frameView) clampOffset(size fyne.Size) {
	clamp := func(off, frame, view float32) float32 {
		margin := min(minDragMargin, frame, view)
		return max(margin-frame, min(view-margin, off))
	}
	f.off.X = clamp(f.off.X, float32(float64(f.img.Bounds().Dx())*f.scale), size.Width)
	f.off.Y = clamp(f.off.Y, float32(float64(f.img.Bounds().Dy())*f.scale), size.Height)
}

// fitScale is the scale at which the frame fits the view.
func (f *frameView) fitScale() float64 {
	size := f.Size()
	w, h := f.img.Bounds().Dx(), f.img.Bounds().Dy()
	if w == 0 || h == 0 || size.Width <= 0 || size.Height <= 0 {
		return 1
	}
	return min(float64(size.Width)/float64(w), float64(size.Height)/float64(h))
}

// zoom multiplies the scale by factor, keeping the frame point at pos in place.
// It zooms out no further than a quarter of the fitted size.
func (f *frameView) zoom(pos fyne.Position, factor float64) {
	if f.scale <= 0 {
		return
	}
	fit := f.fitScale()
	scale := max(fit/4, min(max(maxFrameScale, fit), f.scale*factor))
	x := (float64(pos.X) - float64(f.off.X)) / f.scale // the frame point at pos
	y := (float64(pos.Y) - float64(f.off.Y)) / f.scale
	f.scale = scale
	f.off = fyne.NewPos(pos.X-float32(x*scale), pos.Y-float32(y*scale))
	f.fitted = false
	f.Refresh()
}

// frameAt returns the row and column of the frame pixel at pos in the view; ok
// is false if pos is off the frame.
func (f *frameView) frameAt(pos fyne.Position) (row, col int, ok bool) {
	if f.scale <= 0 {
		return 0, 0, false
	}
	col = int(math.Floor((float64(pos.X) - float64(f.off.X)) / f.scale))
	row = int(math.Floor((float64(pos.Y) - float64(f.off.Y)) / f.scale))
	if !image.Pt(col, row).In(f.img.Bounds()) {
		return 0, 0, false
	}
	return row, col, true
}

// aperturesAt returns the apertures whose boxes, edges included, hold the frame
// pixel at (row, col).
func (f *frameView) aperturesAt(row, col int) []pymoviefile.Aperture {
	var in []pymoviefile.Aperture
	for _, a := range f.apertures {
		if col >= a.X0 && col < a.X0+a.Width && row >= a.Y0 && row < a.Y0+a.Height {
			in = append(in, a)
		}
	}
	return in
}

func (f *frameView) Scrolled(e *fyne.ScrollEvent) {
	f.pointer, f.pointerIn = e.Position, true
	f.zoom(e.Position, math.Pow(zoomPerScroll, float64(e.Scrolled.DY)/scrollNotch))
	f.hover()
}

func (f *frameView) Dragged(e *fyne.DragEvent) {
	f.off = f.off.AddXY(e.Dragged.DX, e.Dragged.DY)
	f.fitted = false
	f.pointer, f.pointerIn = e.Position, true
	f.Refresh()
	f.hover()
}

func (f *frameView) DragEnd() {}

// DoubleTapped fits the frame to the view again.
func (f *frameView) DoubleTapped(*fyne.PointEvent) { f.fitToView() }

// TappedSecondary (a right click) fits the frame to the view again.
func (f *frameView) TappedSecondary(*fyne.PointEvent) { f.fitToView() }

// fitToView fits the frame to the view again, undoing any zoom and drag.
func (f *frameView) fitToView() {
	f.fitted = true
	f.Refresh()
	f.hover()
}

func (f *frameView) Cursor() desktop.Cursor { return desktop.CrosshairCursor }

func (f *frameView) MouseIn(e *desktop.MouseEvent) { f.MouseMoved(e) }
func (f *frameView) MouseMoved(e *desktop.MouseEvent) {
	f.pointer, f.pointerIn = e.Position, true
	f.hover()
}
func (f *frameView) MouseOut() {
	f.pointerIn = false
	f.hover()
}

// hover reports the pixel under the pointer and shows the balloon for the
// apertures it is in, if any.
func (f *frameView) hover() {
	row, col, ok := f.frameAt(f.pointer)
	ok = ok && f.pointerIn
	if f.onHover != nil {
		f.onHover(row, col, ok)
	}
	var names []string
	if ok {
		for _, a := range f.aperturesAt(row, col) {
			names = append(names, a.Name)
		}
	}
	if len(names) == 0 {
		f.balloon.Hide()
		f.balloonText.Hide()
		return
	}
	f.balloonText.Text = strings.Join(names, ", ")
	f.balloon.StrokeColor = apertureBoxColor(f.aperturesAt(row, col)[0].Color)
	f.balloon.Show()
	f.balloonText.Show()
	f.layoutBalloon()
	f.balloon.Refresh()
	f.balloonText.Refresh()
}

// layoutBalloon places the balloon below and to the right of the pointer, or
// on whichever side keeps it in the view.
func (f *frameView) layoutBalloon() {
	text := f.balloonText.MinSize()
	size := text.AddWidthHeight(2*balloonPad, 2*balloonPad)
	view := f.Size()
	pos := f.pointer.AddXY(balloonGap, balloonGap)
	if pos.X+size.Width > view.Width {
		pos.X = f.pointer.X - balloonGap - size.Width
	}
	if pos.Y+size.Height > view.Height {
		pos.Y = f.pointer.Y - balloonGap - size.Height
	}
	pos = fyne.NewPos(max(0, pos.X), max(0, pos.Y))
	f.balloon.Resize(size)
	f.balloon.Move(pos)
	f.balloonText.Resize(text)
	f.balloonText.Move(pos.AddXY(balloonPad, balloonPad))
}

// render draws the part of the frame in view into a w×h pixel image. Zoomed in,
// each output pixel takes the frame pixel under its centre, so frame pixels
// stay square. Zoomed out, an output pixel covers several frame pixels: it
// takes the first colored one among them, so the 1 pixel wide aperture box
// outlines are never lost, or else their average.
func (f *frameView) render(w, h int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	size := f.Size()
	if w <= 0 || h <= 0 || size.Width <= 0 || f.scale <= 0 {
		return dst
	}
	d := float64(w) / float64(size.Width) // output pixels per view unit
	xlo, xhi := sourceSpans(w, d, float64(f.off.X), f.scale, f.img.Bounds().Dx())
	ylo, yhi := sourceSpans(h, d, float64(f.off.Y), f.scale, f.img.Bounds().Dy())
	src := f.img
	for j := range h {
		if ylo[j] >= yhi[j] {
			continue
		}
		for i := range w {
			if xlo[i] >= xhi[i] {
				continue
			}
			var c color.NRGBA
			if yhi[j]-ylo[j] == 1 && xhi[i]-xlo[i] == 1 {
				c = src.NRGBAAt(xlo[i], ylo[j])
			} else {
				c = blockColor(src, xlo[i], xhi[i], ylo[j], yhi[j])
			}
			dst.SetNRGBA(i, j, c)
		}
	}
	return dst
}

// sourceSpans returns, for each of n output pixels along one axis, the frame
// pixels [lo, hi) it covers, given d output pixels per view unit, the frame's
// offset in the view, its scale, and its length in pixels. lo == hi where the
// output pixel is off the frame.
func sourceSpans(n int, d, off, scale float64, length int) (lo, hi []int) {
	lo, hi = make([]int, n), make([]int, n)
	for i := range n {
		a := (float64(i)/d - off) / scale
		b := (float64(i+1)/d - off) / scale
		if b-a <= 1 { // zoomed in: the pixel under the centre
			p := int(math.Floor((a + b) / 2))
			if p >= 0 && p < length {
				lo[i], hi[i] = p, p+1
			}
			continue
		}
		lo[i] = max(0, int(math.Floor(a)))
		hi[i] = max(lo[i], min(length, int(math.Ceil(b))))
	}
	return lo, hi
}

// blockColor returns the first colored pixel in the block of src with columns
// [x0, x1) and rows [y0, y1), or if all are gray, their average.
func blockColor(src *image.NRGBA, x0, x1, y0, y1 int) color.NRGBA {
	var sum, count int
	for y := y0; y < y1; y++ {
		row := src.Pix[src.PixOffset(x0, y):src.PixOffset(x1, y)]
		for k := 0; k < len(row); k += 4 {
			r, g, b := row[k], row[k+1], row[k+2]
			if r != g || g != b {
				return color.NRGBA{R: r, G: g, B: b, A: 255}
			}
			sum += int(r)
			count++
		}
	}
	v := uint8(sum / max(count, 1))
	return color.NRGBA{R: v, G: v, B: v, A: 255}
}
