package main

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// plotWidget shows the rendered light curve plot. Clicking it takes the keyboard
// focus, so it gets the arrow keys, and reports the frame clicked.
type plotWidget struct {
	widget.BaseWidget
	raster *canvas.Raster

	settings func() plotSettings
	canvas   fyne.Canvas              // of the window the plot is in
	onTap    func(frame float64)      // frame at the clicked x position
	onKey    func(key *fyne.KeyEvent) // key pressed while focused

	axis     xAxisPixels // from the last render, in raster pixels
	rasterW  int         // raster width in pixels at the last render
	xMin     float64     // settings().XMin at the last render
	xMax     float64
	rendered bool
}

func newPlotWidget(settings func() plotSettings, c fyne.Canvas) *plotWidget {
	p := &plotWidget{settings: settings, canvas: c}
	p.raster = canvas.NewRaster(p.render)
	p.ExtendBaseWidget(p)
	return p
}

func (p *plotWidget) render(w, h int) image.Image {
	s := p.settings()
	img, axis := renderPlot(s, w, h, p.canvas.Scale())
	p.axis, p.rasterW, p.xMin, p.xMax, p.rendered = axis, w, s.XMin, s.XMax, true
	return img
}

// redraw renders the plot again with the current settings.
func (p *plotWidget) redraw() { p.raster.Refresh() }

func (p *plotWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.raster)
}

func (p *plotWidget) MinSize() fyne.Size { return fyne.NewSize(300, 200) }

// Tapped focuses the plot and reports the frame at the clicked x position.
func (p *plotWidget) Tapped(e *fyne.PointEvent) {
	p.canvas.Focus(p)
	width := p.Size().Width
	if !p.rendered || p.onTap == nil || width <= 0 || p.axis.Right <= p.axis.Left {
		return
	}
	px := float64(e.Position.X) * float64(p.rasterW) / float64(width)
	frac := (px - p.axis.Left) / (p.axis.Right - p.axis.Left)
	p.onTap(p.xMin + frac*(p.xMax-p.xMin))
}

func (p *plotWidget) FocusGained()   {}
func (p *plotWidget) FocusLost()     {}
func (p *plotWidget) TypedRune(rune) {}

func (p *plotWidget) TypedKey(key *fyne.KeyEvent) {
	if p.onKey != nil {
		p.onKey(key)
	}
}
