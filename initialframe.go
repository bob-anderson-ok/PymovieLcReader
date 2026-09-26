package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"pymoviefile"
)

// apertureBoxColors are the colors PyMovie draws aperture boxes in, by color
// name, as the format describes; any other name is drawn magenta.
var apertureBoxColors = map[string]color.NRGBA{
	"red":    {R: 255, A: 255},
	"green":  {G: 255, A: 255},
	"yellow": {R: 255, G: 255, A: 255},
	"white":  {R: 255, G: 255, B: 255, A: 255},
}

var unknownBoxColor = color.NRGBA{R: 255, B: 255, A: 255}

func apertureBoxColor(name string) color.NRGBA {
	if c, ok := apertureBoxColors[name]; ok {
		return c
	}
	return unknownBoxColor
}

// The initial frame window starts big enough to show the frame at full size,
// up to this size.
const (
	maxFrameWindowWidth  = 1200
	maxFrameWindowHeight = 900
)

// hasInitialFrame reports whether the file recorded an initial frame or apertures.
func hasInitialFrame(h pymoviefile.Header) bool { return h.FrameWidth > 0 || len(h.Apertures) > 0 }

// frameSizeText describes the initial frame's size for the header form.
func frameSizeText(h pymoviefile.Header) string {
	if h.FrameWidth == 0 {
		return "not recorded"
	}
	return fmt.Sprintf("%d × %d pixels", h.FrameWidth, h.FrameHeight)
}

// newFrameWindow creates, but does not show, the window for the header's
// initial frame section: the frame with its colored aperture boxes, and a table
// of the apertures' names, colors, boxes and centroids.
func newFrameWindow(app fyne.App, h pymoviefile.Header, title string) fyne.Window {
	w := app.NewWindow(title)
	var frame fyne.CanvasObject = centeredLabel("No frame was recorded in this file.")
	if h.FrameWidth > 0 {
		img := canvas.NewImageFromImage(frameImage(h))
		// Smooth scaling (the default): the box outlines are 1 pixel wide, and
		// scaling down by dropping pixels could lose a box's edge.
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(200, 150))
		frame = img
	}
	top := widget.NewLabel("Initial frame: " + frameSizeText(h))
	w.SetContent(container.NewBorder(top, apertureTable(h.Apertures), nil, nil, frame))
	return w
}

// frameImage returns the header's initial frame as an image.
func frameImage(h pymoviefile.Header) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, h.FrameWidth, h.FrameHeight))
	for row := range h.FrameHeight {
		for col := range h.FrameWidth {
			r, g, b := h.FramePixel(row, col)
			img.SetNRGBA(col, row, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// apertureTable returns a table with a row per aperture: its name beside a
// swatch of its box color, its color, box and centroid.
func apertureTable(apertures []pymoviefile.Aperture) fyne.CanvasObject {
	if len(apertures) == 0 {
		return widget.NewLabel("No apertures were recorded.")
	}
	heading := container.NewGridWithColumns(4,
		boldLabel("Aperture"), boldLabel("Color"), boldLabel("Box corner (x, y), size"), boldLabel("Centroid (x, y)"),
	)
	// The rows are a grid of their own, so that with many apertures they can
	// scroll under the heading; equal-width columns keep the two lined up.
	grid := container.NewGridWithColumns(4)
	for _, a := range apertures {
		swatch := canvas.NewRectangle(color.Transparent)
		swatch.StrokeColor = apertureBoxColor(a.Color)
		swatch.StrokeWidth = 2
		swatch.SetMinSize(fyne.NewSize(16, 16))
		grid.Add(container.NewHBox(container.NewCenter(swatch), widget.NewLabel(a.Name)))
		grid.Add(widget.NewLabel(a.Color))
		grid.Add(widget.NewLabel(fmt.Sprintf("(%d, %d), %d × %d", a.X0, a.Y0, a.Width, a.Height)))
		grid.Add(widget.NewLabel(centroidText(a)))
	}
	return container.NewVBox(heading, scrollIfMany(grid, len(apertures)))
}

func centroidText(a pymoviefile.Aperture) string {
	if math.IsNaN(a.Xc) || math.IsNaN(a.Yc) {
		return "not known"
	}
	return fmt.Sprintf("(%.2f, %.2f)", a.Xc, a.Yc)
}
