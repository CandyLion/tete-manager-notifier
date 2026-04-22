package web

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
	"github.com/wen-ryon/tete-manager-notifier/internal/models"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	staticMapTileSize    = 256
	staticMapMaxZoom     = 18
	staticMapCacheDir    = "tete-manager-notifier-map-cache"
	staticMapTileTimeout = 12 * time.Second
)

type staticTileFetcher struct {
	client      *http.Client
	cacheDir    string
	cacheTTL    time.Duration
	tileURL     string
	userAgent   string
	providerKey string
}

func renderOSMTrackImage(cfg *config.Config, positions []models.Position) (*image.NRGBA, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if strings.TrimSpace(cfg.OSMTileURL) == "" {
		return nil, errors.New("map tile url is empty")
	}
	if len(positions) == 0 {
		return nil, errors.New("no positions")
	}

	zoom := chooseStaticMapZoom(positions, trackImageWidth, trackImageHeight, trackImagePad, staticMapMaxZoom)
	minX, maxX, minY, maxY := mercatorBounds(positions, zoom)
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2
	worldTiles := 1 << uint(zoom)
	worldSize := float64(staticMapTileSize * worldTiles)

	originX := centerX - float64(trackImageWidth)/2
	originY := centerY - float64(trackImageHeight)/2
	if worldSize > float64(trackImageHeight) {
		originY = clampFloat(originY, 0, worldSize-float64(trackImageHeight))
	} else {
		originY = 0
	}

	img := image.NewNRGBA(image.Rect(0, 0, trackImageWidth, trackImageHeight))
	fetcher := newStaticTileFetcher(cfg)
	drawnTiles := 0

	startTileX := int(math.Floor(originX / staticMapTileSize))
	endTileX := int(math.Floor((originX + float64(trackImageWidth)) / staticMapTileSize))
	startTileY := int(math.Floor(originY / staticMapTileSize))
	endTileY := int(math.Floor((originY + float64(trackImageHeight)) / staticMapTileSize))

	for tileY := startTileY; tileY <= endTileY; tileY++ {
		for tileX := startTileX; tileX <= endTileX; tileX++ {
			destRect := image.Rect(
				int(float64(tileX*staticMapTileSize)-originX),
				int(float64(tileY*staticMapTileSize)-originY),
				int(float64((tileX+1)*staticMapTileSize)-originX),
				int(float64((tileY+1)*staticMapTileSize)-originY),
			)

			tileImg, err := fetcher.fetchTile(zoom, tileX, tileY)
			if err != nil {
				drawMissingTile(img, destRect)
				continue
			}

			imagedraw.Draw(img, destRect, tileImg, image.Point{}, imagedraw.Src)
			drawnTiles++
		}
	}

	if drawnTiles == 0 {
		return nil, errors.New("no map tiles fetched")
	}

	applyTint(img, color.NRGBA{R: 7, G: 18, B: 26, A: 34})

	points := projectTrackMercator(positions, zoom, originX, originY)
	if len(points) == 0 {
		return nil, errors.New("failed to project track")
	}

	drawPath(img, points, color.NRGBA{R: 25, G: 54, B: 73, A: 120}, 14)
	drawPath(img, points, color.NRGBA{R: 72, G: 187, B: 255, A: 84}, 10)
	drawPath(img, points, color.NRGBA{R: 72, G: 197, B: 255, A: 196}, 6)
	drawPath(img, points, color.NRGBA{R: 245, G: 252, B: 255, A: 255}, 2)
	drawProgressDots(img, points)

	start := points[0]
	end := points[len(points)-1]
	drawMarker(img, start, color.NRGBA{R: 72, G: 214, B: 138, A: 255})
	drawMarker(img, end, color.NRGBA{R: 255, G: 130, B: 92, A: 255})
	drawAttribution(img, "© OpenStreetMap contributors")

	return img, nil
}

func newStaticTileFetcher(cfg *config.Config) *staticTileFetcher {
	cacheTTL := time.Duration(cfg.OSMTileCacheHours) * time.Hour
	if cacheTTL <= 0 {
		cacheTTL = 168 * time.Hour
	}

	urlTemplate := strings.TrimSpace(cfg.OSMTileURL)
	sum := sha1.Sum([]byte(urlTemplate))
	providerKey := hex.EncodeToString(sum[:6])

	return &staticTileFetcher{
		client: &http.Client{
			Timeout: staticMapTileTimeout,
		},
		cacheDir:    filepath.Join(os.TempDir(), staticMapCacheDir),
		cacheTTL:    cacheTTL,
		tileURL:     urlTemplate,
		userAgent:   strings.TrimSpace(cfg.OSMTileUserAgent),
		providerKey: providerKey,
	}
}

func (f *staticTileFetcher) fetchTile(zoom, tileX, tileY int) (image.Image, error) {
	worldTiles := 1 << zoom
	if tileY < 0 || tileY >= worldTiles {
		return nil, errors.New("tile y out of range")
	}

	wrappedX := wrapTileIndex(tileX, worldTiles)
	cachePath := f.tileCachePath(zoom, wrappedX, tileY)

	if cached, err := f.loadCachedTile(cachePath, false); err == nil {
		return cached, nil
	}

	url := strings.NewReplacer(
		"{z}", strconv.Itoa(zoom),
		"{x}", strconv.Itoa(wrappedX),
		"{y}", strconv.Itoa(tileY),
	).Replace(f.tileURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}
	req.Header.Set("Accept", "image/png,image/*;q=0.9,*/*;q=0.8")

	resp, err := f.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			if readErr == nil {
				img, _, decodeErr := image.Decode(bytes.NewReader(body))
				if decodeErr == nil {
					_ = f.storeTile(cachePath, body)
					return img, nil
				}
				err = decodeErr
			} else {
				err = readErr
			}
		} else {
			err = fmt.Errorf("tile status=%d", resp.StatusCode)
		}
	}

	if cached, staleErr := f.loadCachedTile(cachePath, true); staleErr == nil {
		return cached, nil
	}

	return nil, err
}

func (f *staticTileFetcher) tileCachePath(zoom, tileX, tileY int) string {
	return filepath.Join(
		f.cacheDir,
		f.providerKey,
		strconv.Itoa(zoom),
		strconv.Itoa(tileX),
		fmt.Sprintf("%d.png", tileY),
	)
}

func (f *staticTileFetcher) loadCachedTile(path string, allowStale bool) (image.Image, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !allowStale && time.Since(info.ModTime()) > f.cacheTTL {
		return nil, errors.New("tile cache expired")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (f *staticTileFetcher) storeTile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func chooseStaticMapZoom(positions []models.Position, width, height int, padding float64, maxZoom int) int {
	if maxZoom < 1 {
		maxZoom = staticMapMaxZoom
	}

	availableWidth := float64(width) - 2*padding
	availableHeight := float64(height) - 2*padding
	if availableWidth < 64 {
		availableWidth = float64(width)
	}
	if availableHeight < 64 {
		availableHeight = float64(height)
	}

	for zoom := maxZoom; zoom >= 1; zoom-- {
		minX, maxX, minY, maxY := mercatorBounds(positions, zoom)
		spanX := math.Max(maxX-minX, 1)
		spanY := math.Max(maxY-minY, 1)
		if spanX <= availableWidth && spanY <= availableHeight {
			return zoom
		}
	}

	return 1
}

func mercatorBounds(positions []models.Position, zoom int) (minX, maxX, minY, maxY float64) {
	x, y := mercatorProject(positions[0].Latitude, positions[0].Longitude, zoom)
	minX, maxX, minY, maxY = x, x, y, y

	for _, p := range positions[1:] {
		x, y = mercatorProject(p.Latitude, p.Longitude, zoom)
		minX = math.Min(minX, x)
		maxX = math.Max(maxX, x)
		minY = math.Min(minY, y)
		maxY = math.Max(maxY, y)
	}

	return minX, maxX, minY, maxY
}

func projectTrackMercator(positions []models.Position, zoom int, originX, originY float64) []projectedPoint {
	projected := make([]projectedPoint, 0, len(positions))
	for _, p := range positions {
		x, y := mercatorProject(p.Latitude, p.Longitude, zoom)
		projected = append(projected, projectedPoint{
			X: x - originX,
			Y: y - originY,
		})
	}
	return projected
}

func mercatorProject(lat, lon float64, zoom int) (float64, float64) {
	worldTiles := 1 << uint(zoom)
	scale := float64(staticMapTileSize * worldTiles)
	x := (lon + 180.0) / 360.0 * scale
	latRad := lat * math.Pi / 180
	sinLat := math.Sin(latRad)
	y := (0.5 - math.Log((1+sinLat)/(1-sinLat))/(4*math.Pi)) * scale
	return x, y
}

func wrapTileIndex(v, mod int) int {
	if mod == 0 {
		return v
	}
	wrapped := v % mod
	if wrapped < 0 {
		wrapped += mod
	}
	return wrapped
}

func clampFloat(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func drawMissingTile(img *image.NRGBA, rect image.Rectangle) {
	fillRect(img, rect, color.NRGBA{R: 221, G: 228, B: 232, A: 255})
	grid := color.NRGBA{R: 196, G: 205, B: 212, A: 255}

	for x := rect.Min.X; x < rect.Max.X; x += 32 {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if image.Pt(x, y).In(img.Bounds()) {
				img.SetNRGBA(x, y, grid)
			}
		}
	}
	for y := rect.Min.Y; y < rect.Max.Y; y += 32 {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if image.Pt(x, y).In(img.Bounds()) {
				img.SetNRGBA(x, y, grid)
			}
		}
	}
}

func applyTint(img *image.NRGBA, tint color.NRGBA) {
	fillRectAlpha(img, img.Bounds(), tint)
}

func fillRect(img *image.NRGBA, rect image.Rectangle, col color.NRGBA) {
	clipped := rect.Intersect(img.Bounds())
	for y := clipped.Min.Y; y < clipped.Max.Y; y++ {
		for x := clipped.Min.X; x < clipped.Max.X; x++ {
			img.SetNRGBA(x, y, col)
		}
	}
}

func fillRectAlpha(img *image.NRGBA, rect image.Rectangle, col color.NRGBA) {
	clipped := rect.Intersect(img.Bounds())
	for y := clipped.Min.Y; y < clipped.Max.Y; y++ {
		for x := clipped.Min.X; x < clipped.Max.X; x++ {
			img.SetNRGBA(x, y, blend(img.NRGBAAt(x, y), col))
		}
	}
}

func drawAttribution(img *image.NRGBA, text string) {
	if text == "" {
		return
	}

	textWidth := 7*len(text) + 12
	barHeight := 22
	rect := image.Rect(
		img.Bounds().Max.X-textWidth-10,
		img.Bounds().Max.Y-barHeight-10,
		img.Bounds().Max.X-10,
		img.Bounds().Max.Y-10,
	)
	fillRectAlpha(img, rect, color.NRGBA{R: 8, G: 22, B: 39, A: 185})

	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.NRGBA{R: 255, G: 255, B: 255, A: 230}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(rect.Min.X+6, rect.Min.Y+15),
	}
	drawer.DrawString(text)
}
