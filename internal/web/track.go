package web

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
	"github.com/wen-ryon/tete-manager-notifier/internal/models"
)

const (
	trackImageWidth  = 1200
	trackImageHeight = 780
	trackImagePad    = 86.0
)

type projectedPoint struct {
	X float64
	Y float64
}

func RenderTrackPNG(cfg *config.Config, positions []models.Position) ([]byte, error) {
	return RenderTrackPNGForProvider(cfg, MapProviderOSM, positions)
}

func RenderTrackPNGForProvider(cfg *config.Config, provider string, positions []models.Position) ([]byte, error) {
	img, data, err := renderTrackForProvider(cfg, provider, positions)
	if err != nil {
		img = renderFallbackTrackImage(positions)
	}

	if len(data) > 0 {
		return data, nil
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderTrackForProvider(cfg *config.Config, provider string, positions []models.Position) (*image.NRGBA, []byte, error) {
	switch resolveTrackProvider(cfg, provider, MapProviderOSM) {
	case MapProviderAMap:
		data, err := renderAMapTrackPNG(cfg, positions)
		if err == nil {
			return nil, data, nil
		}
		img, osmErr := renderOSMTrackImage(cfg, positions)
		if osmErr == nil {
			return img, nil, nil
		}
		return nil, nil, err
	case MapProviderOSM:
		img, err := renderOSMTrackImage(cfg, positions)
		return img, nil, err
	default:
		return nil, nil, image.ErrFormat
	}
}

func renderFallbackTrackImage(positions []models.Position) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, trackImageWidth, trackImageHeight))

	paintBackground(img)
	drawGrid(img)

	points := projectTrack(positions, trackImageWidth, trackImageHeight, trackImagePad)
	if len(points) == 0 {
		return img
	}

	drawPath(img, points, color.NRGBA{R: 72, G: 187, B: 255, A: 70}, 12)
	drawPath(img, points, color.NRGBA{R: 72, G: 187, B: 255, A: 170}, 7)
	drawPath(img, points, color.NRGBA{R: 237, G: 248, B: 255, A: 255}, 3)
	drawProgressDots(img, points)

	start := points[0]
	end := points[len(points)-1]
	drawMarker(img, start, color.NRGBA{R: 72, G: 214, B: 138, A: 255})
	drawMarker(img, end, color.NRGBA{R: 255, G: 130, B: 92, A: 255})
	return img
}

func SamplePositions(positions []models.Position, maxPoints int) []models.Position {
	if maxPoints <= 1 {
		if len(positions) == 0 {
			return nil
		}
		return []models.Position{positions[0]}
	}

	if len(positions) <= maxPoints {
		return positions
	}

	sampled := make([]models.Position, 0, maxPoints)
	lastIndex := len(positions) - 1
	lastUsedIndex := -1

	for i := 0; i < maxPoints; i++ {
		index := int(math.Round(float64(i) * float64(lastIndex) / float64(maxPoints-1)))
		if index < 0 {
			index = 0
		}
		if index > lastIndex {
			index = lastIndex
		}
		if index == lastUsedIndex {
			continue
		}
		sampled = append(sampled, positions[index])
		lastUsedIndex = index
	}

	if lastUsedIndex != lastIndex {
		sampled = append(sampled, positions[lastIndex])
	}

	return sampled
}

func FilterTrackPositions(positions []models.Position) []models.Position {
	filtered := make([]models.Position, 0, len(positions))
	for _, p := range positions {
		if p.Latitude == 0 && p.Longitude == 0 {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

func projectTrack(positions []models.Position, width, height int, padding float64) []projectedPoint {
	if len(positions) == 0 {
		return nil
	}

	minLat, maxLat := positions[0].Latitude, positions[0].Latitude
	minLon, maxLon := positions[0].Longitude, positions[0].Longitude

	for _, p := range positions[1:] {
		minLat = math.Min(minLat, p.Latitude)
		maxLat = math.Max(maxLat, p.Latitude)
		minLon = math.Min(minLon, p.Longitude)
		maxLon = math.Max(maxLon, p.Longitude)
	}

	lonSpan := maxLon - minLon
	latSpan := maxLat - minLat
	if lonSpan == 0 {
		lonSpan = 0.0001
	}
	if latSpan == 0 {
		latSpan = 0.0001
	}

	scaleX := (float64(width) - 2*padding) / lonSpan
	scaleY := (float64(height) - 2*padding) / latSpan
	scale := math.Min(scaleX, scaleY)

	contentWidth := lonSpan * scale
	contentHeight := latSpan * scale
	offsetX := (float64(width)-contentWidth)/2 - minLon*scale
	offsetY := (float64(height)-contentHeight)/2 + maxLat*scale

	projected := make([]projectedPoint, 0, len(positions))
	for _, p := range positions {
		projected = append(projected, projectedPoint{
			X: p.Longitude*scale + offsetX,
			Y: offsetY - p.Latitude*scale,
		})
	}

	return projected
}

func paintBackground(img *image.NRGBA) {
	bounds := img.Bounds()
	top := color.NRGBA{R: 8, G: 22, B: 39, A: 255}
	bottom := color.NRGBA{R: 16, G: 54, B: 75, A: 255}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		t := float64(y-bounds.Min.Y) / float64(bounds.Dy())
		line := color.NRGBA{
			R: uint8(float64(top.R)*(1-t) + float64(bottom.R)*t),
			G: uint8(float64(top.G)*(1-t) + float64(bottom.G)*t),
			B: uint8(float64(top.B)*(1-t) + float64(bottom.B)*t),
			A: 255,
		}

		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetNRGBA(x, y, line)
		}
	}
}

func drawGrid(img *image.NRGBA) {
	bounds := img.Bounds()
	gridColor := color.NRGBA{R: 255, G: 255, B: 255, A: 14}

	for x := bounds.Min.X + 60; x < bounds.Max.X; x += 60 {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			img.SetNRGBA(x, y, blend(img.NRGBAAt(x, y), gridColor))
		}
	}

	for y := bounds.Min.Y + 60; y < bounds.Max.Y; y += 60 {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetNRGBA(x, y, blend(img.NRGBAAt(x, y), gridColor))
		}
	}
}

func drawPath(img *image.NRGBA, points []projectedPoint, col color.NRGBA, radius int) {
	if len(points) == 1 {
		drawFilledCircle(img, int(points[0].X), int(points[0].Y), radius, col)
		return
	}

	for i := 1; i < len(points); i++ {
		drawThickLine(img, points[i-1], points[i], radius, col)
	}
}

func drawMarker(img *image.NRGBA, point projectedPoint, accent color.NRGBA) {
	drawFilledCircle(img, int(point.X), int(point.Y), 15, color.NRGBA{R: 255, G: 255, B: 255, A: 50})
	drawFilledCircle(img, int(point.X), int(point.Y), 11, accent)
	drawFilledCircle(img, int(point.X), int(point.Y), 4, color.NRGBA{R: 250, G: 251, B: 252, A: 255})
}

func drawProgressDots(img *image.NRGBA, points []projectedPoint) {
	if len(points) < 6 {
		return
	}

	step := len(points) / 12
	if step < 2 {
		step = 2
	}

	for i := step; i < len(points)-1; i += step {
		drawFilledCircle(img, int(points[i].X), int(points[i].Y), 4, color.NRGBA{R: 255, G: 255, B: 255, A: 34})
	}
}

func drawThickLine(img *image.NRGBA, from, to projectedPoint, radius int, col color.NRGBA) {
	dx := to.X - from.X
	dy := to.Y - from.Y
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps == 0 {
		drawFilledCircle(img, int(from.X), int(from.Y), radius, col)
		return
	}

	for step := 0; step <= steps; step++ {
		t := float64(step) / float64(steps)
		x := from.X + dx*t
		y := from.Y + dy*t
		drawFilledCircle(img, int(math.Round(x)), int(math.Round(y)), radius, col)
	}
}

func drawFilledCircle(img *image.NRGBA, cx, cy, radius int, col color.NRGBA) {
	radiusSq := radius * radius
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			if !image.Pt(x, y).In(img.Bounds()) {
				continue
			}
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy > radiusSq {
				continue
			}
			img.SetNRGBA(x, y, blend(img.NRGBAAt(x, y), col))
		}
	}
}

func blend(dst, src color.NRGBA) color.NRGBA {
	srcAlpha := float64(src.A) / 255
	dstAlpha := float64(dst.A) / 255
	outAlpha := srcAlpha + dstAlpha*(1-srcAlpha)
	if outAlpha == 0 {
		return color.NRGBA{}
	}

	return color.NRGBA{
		R: uint8((float64(src.R)*srcAlpha + float64(dst.R)*dstAlpha*(1-srcAlpha)) / outAlpha),
		G: uint8((float64(src.G)*srcAlpha + float64(dst.G)*dstAlpha*(1-srcAlpha)) / outAlpha),
		B: uint8((float64(src.B)*srcAlpha + float64(dst.B)*dstAlpha*(1-srcAlpha)) / outAlpha),
		A: uint8(outAlpha * 255),
	}
}
