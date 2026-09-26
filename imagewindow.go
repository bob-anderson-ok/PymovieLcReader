package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// maskClosedColor fills the pixels outside the sampling mask. It is not a gray,
// so it can't be mistaken for image data.
var maskClosedColor = color.NRGBA{R: 0xff, G: 0x8c, B: 0x00, A: 0xff} // orange

// highPixelColor marks pixels at or above the level the user gives.
var highPixelColor = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff} // red

// imageWindow shows one aperture's image at the cursor frame: the raw image on
// the left, and on the right the image data inside the sampling mask. Two
// sliders set the black and white levels used for both. Pointing at a pixel in
// either image shows its value.
type imageWindow struct {
	win   fyne.Window
	curve *lightCurve
	point int // index of the point shown, or -1 if the curve has none at the frame

	black, white float64
	red          float64 // pixels at or above this are red; +Inf for none
	raw, masked  *pixelView
	info         *widget.Label

	pixelInfo          *widget.Label // the value of the pixel under the pointer
	hovering           bool
	hoverRow, hoverCol int
}

const pixelInfoPrompt = "Point at a pixel to see its value"

// newImageWindow creates the window for curve's images. It is not shown yet.
func newImageWindow(app fyne.App, curve *lightCurve) *imageWindow {
	iw := &imageWindow{
		win:       app.NewWindow(curve.Name + " - aperture images"),
		curve:     curve,
		point:     -1,
		red:       math.Inf(1),
		info:      widget.NewLabel(""),
		raw:       newPixelView(),
		masked:    newPixelView(),
		pixelInfo: widget.NewLabel(pixelInfoPrompt),
	}
	iw.raw.onHover = iw.hover
	iw.masked.onHover = iw.hover
	lo, hi := curve.pixelRange()
	iw.black, iw.white = float64(lo), float64(hi)

	images := container.NewGridWithColumns(2,
		container.NewBorder(centeredLabel("Raw image"), nil, nil, nil, iw.raw),
		container.NewBorder(centeredLabel("Sampling mask"), nil, nil, nil, iw.masked),
	)
	sliderMax := max(float64(hi), 1)
	levels := widget.NewForm(
		widget.NewFormItem("Black level", iw.levelSlider(&iw.black, sliderMax)),
		widget.NewFormItem("White level", iw.levelSlider(&iw.white, sliderMax)),
	)
	bottom := container.NewVBox(iw.pixelInfo, levels)
	iw.win.SetContent(container.NewBorder(iw.info, bottom, nil, nil, images))
	return iw
}

// hover records the pixel under the pointer (ok false: none) and shows its value.
func (iw *imageWindow) hover(row, col int, ok bool) {
	iw.hovering, iw.hoverRow, iw.hoverCol = ok, row, col
	iw.showPixelInfo()
}

// showPixelInfo shows the position, value and mask state of the pixel under the
// pointer, in the frame shown.
func (iw *imageWindow) showPixelInfo() {
	if !iw.hovering || iw.point < 0 {
		iw.pixelInfo.SetText(pixelInfoPrompt)
		return
	}
	i := iw.hoverRow*iw.curve.RoiSize + iw.hoverCol
	text := fmt.Sprintf("Pixel x %d, y %d:  %d", iw.hoverCol, iw.hoverRow, iw.curve.Images[iw.point][i])
	if iw.curve.Masks[iw.point][i] != 0 {
		text += "   (in the sampling mask)"
	} else {
		text += "   (outside the sampling mask)"
	}
	iw.pixelInfo.SetText(text)
}

// levelSlider returns a slider, with its value shown beside it, that sets *level.
func (iw *imageWindow) levelSlider(level *float64, maxValue float64) fyne.CanvasObject {
	value := widget.NewLabel("")
	slider := widget.NewSlider(0, maxValue)
	slider.SetValue(*level)
	value.SetText(fmt.Sprint(int(*level)))
	slider.OnChanged = func(x float64) {
		*level = x
		value.SetText(fmt.Sprint(int(x)))
		iw.render()
	}
	return container.NewBorder(nil, nil, nil, value, slider)
}

// showFrame shows the images at frame, the cursor frame of the plot, and the
// light curve value there: the appsum or the intensity, as on the plot.
func (iw *imageWindow) showFrame(frame float64, useAppsum bool) {
	iw.point = -1
	text := fmt.Sprintf("Frame %g", frame)
	if i, ok := iw.curve.pointAt(frame); ok {
		iw.point = i
		if ts := iw.curve.Timestamps[i]; ts != "" {
			text += "   " + ts
		}
		field := "Intensity"
		if useAppsum {
			field = "Appsum"
		}
		text += fmt.Sprintf("   %s: %.6g", field, iw.curve.values(useAppsum)[i])
	} else {
		text += "   (no record for this aperture)"
	}
	iw.info.SetText(text)
	iw.render()
}

// setRedLevel colors pixels at or above level red; +Inf colors none.
func (iw *imageWindow) setRedLevel(level float64) {
	iw.red = level
	iw.render()
}

func (iw *imageWindow) render() {
	n := iw.curve.RoiSize
	if iw.point < 0 || n == 0 {
		iw.raw.setImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)), 0)
		iw.masked.setImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)), 0)
	} else {
		img, mask := iw.curve.Images[iw.point], iw.curve.Masks[iw.point]
		iw.raw.setImage(scaledImage(img, nil, n, iw.black, iw.white, iw.red), n)
		iw.masked.setImage(scaledImage(img, mask, n, iw.black, iw.white, iw.red), n)
	}
	iw.showPixelInfo() // the pixel under the pointer may now be in another frame
}

// scaledImage turns an n×n row-major aperture image into gray levels: values at
// or below black are black, at or above white are white, and linear between.
// Pixels at or above red are highPixelColor. If mask is not nil, pixels where
// it is 0 are filled with maskClosedColor instead.
func scaledImage(pixels []uint16, mask []uint8, n int, black, white, red float64) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, n, n))
	span := white - black
	for row := range n {
		for col := range n {
			i := row*n + col
			if mask != nil && mask[i] == 0 {
				out.SetNRGBA(col, row, maskClosedColor)
				continue
			}
			if float64(pixels[i]) >= red {
				out.SetNRGBA(col, row, highPixelColor)
				continue
			}
			var g uint8
			switch v := float64(pixels[i]); {
			case span <= 0: // levels crossed or equal: a hard threshold at black
				if v > black {
					g = 255
				}
			default:
				g = uint8(255*min(max((v-black)/span, 0), 1) + 0.5)
			}
			out.SetNRGBA(col, row, color.NRGBA{R: g, G: g, B: g, A: 0xff})
		}
	}
	return out
}

func centeredLabel(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignCenter, fyne.TextStyle{})
}
