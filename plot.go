package main

import (
	"image"
	"image/color"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// plotSettings is everything renderPlot needs to draw the light curves.
type plotSettings struct {
	Curves     []*lightCurve
	Colors     []color.Color // Colors[i] is the color of Curves[i]
	UseAppsum  bool
	XMin, XMax float64
	YMin, YMax float64     // ignored unless YMin < YMax
	DotSize    float64     // dot diameter in device-independent pixels
	Ticker     *timeTicker // nil: label the x axis with frame numbers
	ShowCursor bool
	Cursor     float64 // frame the vertical cursor line is drawn at
}

// xAxisPixels gives the horizontal pixel positions of XMin and XMax in a
// rendered plot, for turning a click position back into a frame.
type xAxisPixels struct{ Left, Right float64 }

var gridColor = color.NRGBA{R: 0xb8, G: 0xb8, B: 0xb8, A: 0xff}

var cursorColor = color.NRGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xff}

// cursorLine is a plotter that draws a vertical line across the whole data area.
type cursorLine struct{ x float64 }

func (l cursorLine) Plot(c draw.Canvas, p *plot.Plot) {
	if l.x < p.X.Min || l.x > p.X.Max {
		return
	}
	xf, _ := p.Transforms(&c)
	x := xf(l.x)
	c.StrokeLine2(draw.LineStyle{Color: cursorColor, Width: vg.Points(1)}, x, c.Min.Y, x, c.Max.Y)
}

// Ticks implements plot.Ticker: the default ticks, labelled with timestamps.
func (t *timeTicker) Ticks(min, max float64) []plot.Tick {
	ticks := plot.DefaultTicks{}.Ticks(min, max)
	for i := range ticks {
		if ticks[i].Label != "" { // minor ticks have no label
			ticks[i].Label = t.stampAt(ticks[i].Value)
		}
	}
	return ticks
}

// renderPlot draws the light curves as dots joined by lines into a w×h pixel
// image, with the cursor line if ShowCursor is set. scale is the Fyne canvas
// scale, so text and dots keep their size on high-DPI screens.
func renderPlot(s plotSettings, w, h int, scale float32) (image.Image, xAxisPixels) {
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), xAxisPixels{}
	}

	p := plot.New()
	p.Y.Label.Text = "Intensity"
	if s.UseAppsum {
		p.Y.Label.Text = "Appsum"
	}
	p.X.Label.Text = "Frame"
	if s.Ticker != nil {
		p.X.Label.Text = "Timestamp"
		p.X.Tick.Marker = s.Ticker
	}
	p.Y.Tick.Marker = plot.DefaultTicks{}
	p.Legend.Top = true

	grid := plotter.NewGrid() // lines at the major ticks, behind the curves
	grid.Vertical.Color = gridColor
	grid.Horizontal.Color = gridColor
	p.Add(grid)

	for i, c := range s.Curves {
		values := c.values(s.UseAppsum)
		xys := make(plotter.XYs, len(c.Frames))
		for k := range xys {
			xys[k] = plotter.XY{X: c.Frames[k], Y: values[k]}
		}
		line, points, err := plotter.NewLinePoints(xys)
		if err != nil { // NaN or infinite values
			continue
		}
		line.Color = s.Colors[i]
		line.Width = vg.Points(1)
		points.Color = s.Colors[i]
		points.Shape = draw.CircleGlyph{}
		points.Radius = vg.Points(s.DotSize / 2 * 72 / vgimg.DefaultDPI)
		p.Add(line, points)
		p.Legend.Add(c.Name, line, points)
	}

	if s.ShowCursor {
		p.Add(cursorLine{x: s.Cursor})
	}

	// Set the ranges after adding the curves, which widen them to fit the data.
	p.X.Min, p.X.Max = s.XMin, s.XMax
	if s.YMin < s.YMax {
		p.Y.Min, p.Y.Max = s.YMin, s.YMax
	}

	dpi := vgimg.DefaultDPI * float64(scale)
	c := vgimg.NewWith(
		vgimg.UseWH(vg.Length(w)*vg.Inch/vg.Length(dpi), vg.Length(h)*vg.Inch/vg.Length(dpi)),
		vgimg.UseDPI(int(dpi+0.5)),
	)
	dc := draw.New(c)
	p.Draw(dc)

	area := p.DataCanvas(dc)
	xf, _ := p.Transforms(&area)
	toPixels := func(l vg.Length) float64 { return float64(l/vg.Inch) * dpi }
	return c.Image(), xAxisPixels{Left: toPixels(xf(s.XMin)), Right: toPixels(xf(s.XMax))}
}
