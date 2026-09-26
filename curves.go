package main

import (
	"math"
	"sort"

	"pymoviefile"
)

// lightCurve holds one aperture's measurements, sorted by frame.
type lightCurve struct {
	Name       string
	Frames     []float64
	Intensity  []float64
	Appsum     []float64
	Timestamps []string
	RoiSize    int        // n: every image and mask is n×n
	Images     [][]uint16 // Images[i] is the aperture image at Frames[i], row-major
	Masks      [][]uint8  // Masks[i] is the sampling mask at Frames[i], row-major, 0 or 1
}

// values returns the appsum or the intensity values.
func (c *lightCurve) values(useAppsum bool) []float64 {
	if useAppsum {
		return c.Appsum
	}
	return c.Intensity
}

// buildLightCurves splits the records into one light curve per aperture, in the
// order the apertures appear in an aperture group. Each curve is sorted by frame,
// since records are in measured order, which differs from frame order when a
// video was analysed backwards or in pieces.
func buildLightCurves(records []pymoviefile.Record) []*lightCurve {
	var curves []*lightCurve
	byName := map[string][]pymoviefile.Record{}
	for _, r := range records {
		if _, seen := byName[r.Name]; !seen {
			curves = append(curves, &lightCurve{Name: r.Name})
		}
		byName[r.Name] = append(byName[r.Name], r)
	}
	for _, c := range curves {
		recs := byName[c.Name]
		sort.SliceStable(recs, func(i, j int) bool { return recs[i].Frame < recs[j].Frame })
		for _, r := range recs {
			c.Frames = append(c.Frames, r.Frame)
			c.Intensity = append(c.Intensity, r.Intensity)
			c.Appsum = append(c.Appsum, r.Appsum)
			c.Timestamps = append(c.Timestamps, r.Timestamp)
			c.Images = append(c.Images, r.Image)
			c.Masks = append(c.Masks, r.Mask)
			c.RoiSize = r.RoiSize
		}
	}
	return curves
}

// defaultYLimits returns the default display limits for the given curves'
// points from frame begin to end: below, 0, or 110% of the smallest value if
// any value is negative; above, 110% of the largest value, or 0 if no value is
// positive (110% of a negative maximum would hide it).
func defaultYLimits(curves []*lightCurve, useAppsum bool, begin, end float64) (lo, hi float64) {
	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, c := range curves {
		for i, v := range c.values(useAppsum) {
			if f := c.Frames[i]; f >= begin && f <= end {
				minV, maxV = math.Min(minV, v), math.Max(maxV, v)
			}
		}
	}
	if math.IsInf(maxV, -1) {
		return 0, 1 // no data
	}
	if minV < 0 {
		lo = 1.1 * minV
	}
	if maxV > 0 {
		hi = 1.1 * maxV
	}
	if hi <= lo {
		hi = lo + 1 // every value is 0
	}
	return lo, hi
}

// frameRange returns the smallest and largest frame in the curves.
func frameRange(curves []*lightCurve) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, c := range curves {
		for _, f := range c.Frames {
			lo, hi = math.Min(lo, f), math.Max(hi, f)
		}
	}
	if lo > hi {
		return 0, 1
	}
	if lo == hi {
		hi = lo + 1
	}
	return lo, hi
}

// timeTicker labels frame-number ticks with the timestamp of the nearest frame.
type timeTicker struct {
	frames []float64 // sorted
	stamps []string  // stamps[i] is the timestamp of frames[i]
}

// newTimeTicker returns a ticker for the curves' timestamps, or nil if any
// record has a blank timestamp, in which case the axis shows frame numbers.
func newTimeTicker(curves []*lightCurve) *timeTicker {
	stampOf := map[float64]string{}
	for _, c := range curves {
		for i, ts := range c.Timestamps {
			if ts == "" {
				return nil
			}
			stampOf[c.Frames[i]] = ts
		}
	}
	if len(stampOf) == 0 {
		return nil
	}
	t := &timeTicker{}
	for f := range stampOf {
		t.frames = append(t.frames, f)
	}
	sort.Float64s(t.frames)
	for _, f := range t.frames {
		t.stamps = append(t.stamps, stampOf[f])
	}
	return t
}

// stampAt returns the timestamp of the frame nearest to x.
func (t *timeTicker) stampAt(x float64) string { return t.stamps[nearestIndex(t.frames, x)] }

// allFrames returns every frame in the curves, sorted, without repeats.
func allFrames(curves []*lightCurve) []float64 {
	seen := map[float64]bool{}
	var frames []float64
	for _, c := range curves {
		for _, f := range c.Frames {
			if !seen[f] {
				seen[f] = true
				frames = append(frames, f)
			}
		}
	}
	sort.Float64s(frames)
	return frames
}

// nearestIndex returns the index of the value in sorted (which must not be
// empty) that is nearest to x.
func nearestIndex(sorted []float64, x float64) int {
	i := sort.SearchFloat64s(sorted, x)
	if i == len(sorted) || (i > 0 && x-sorted[i-1] < sorted[i]-x) {
		i--
	}
	return i
}

// valueAt returns the curve's value at frame, and false if the curve has no
// point there.
func (c *lightCurve) valueAt(frame float64, useAppsum bool) (float64, bool) {
	i, ok := c.pointAt(frame)
	if !ok {
		return 0, false
	}
	return c.values(useAppsum)[i], true
}

// pointAt returns the index of the curve's point at frame, and false if the
// curve has no point there.
func (c *lightCurve) pointAt(frame float64) (int, bool) {
	i := sort.SearchFloat64s(c.Frames, frame)
	return i, i < len(c.Frames) && c.Frames[i] == frame
}

// pixelRange returns the smallest and largest pixel value in all the curve's images.
func (c *lightCurve) pixelRange() (lo, hi uint16) {
	lo = math.MaxUint16
	for _, img := range c.Images {
		for _, p := range img {
			lo, hi = min(lo, p), max(hi, p)
		}
	}
	if lo > hi {
		return 0, 0
	}
	return lo, hi
}

// saturationLevel returns the saturation value in effect in the records: the
// most common non-zero one, as it may change during a run (the smaller on a
// tie), or 0 if every record has 0.
func saturationLevel(records []pymoviefile.Record) uint16 {
	count := map[uint16]int{}
	var level uint16
	for _, r := range records {
		if r.Saturation == 0 {
			continue
		}
		count[r.Saturation]++
		if n, best := count[r.Saturation], count[level]; n > best || (n == best && r.Saturation < level) {
			level = r.Saturation
		}
	}
	return level
}
