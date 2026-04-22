package web

import (
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
	"github.com/wen-ryon/tete-manager-notifier/internal/models"
)

const (
	aMapStaticWidth     = 600
	aMapStaticHeight    = 390
	aMapStaticScale     = 2
	aMapStaticTimeout   = 12 * time.Second
	aMapStaticMaxPoints = 80
)

func renderAMapTrackPNG(cfg *config.Config, positions []models.Position) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.AMapStaticMapURL) == "" {
		return nil, fmt.Errorf("amap static map url is empty")
	}
	if strings.TrimSpace(cfg.AMapWebServiceKey) == "" {
		return nil, fmt.Errorf("amap web service key is empty")
	}
	if len(positions) == 0 {
		return nil, fmt.Errorf("no positions")
	}

	converted := SamplePositions(positionsForProvider(MapProviderAMap, positions), aMapStaticMaxPoints)
	if len(converted) == 0 {
		return nil, fmt.Errorf("no converted positions")
	}

	values := neturl.Values{}
	values.Set("key", cfg.AMapWebServiceKey)
	values.Set("size", fmt.Sprintf("%d*%d", aMapStaticWidth, aMapStaticHeight))
	values.Set("scale", fmt.Sprintf("%d", aMapStaticScale))
	values.Set("markers", buildAMapMarkers(converted))
	values.Set("paths", buildAMapPath(converted))

	req, err := http.NewRequest(http.MethodGet, strings.TrimSpace(cfg.AMapStaticMapURL)+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/png,image/*;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", aMapUserAgent(cfg))

	client := &http.Client{Timeout: aMapStaticTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("amap status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if strings.Contains(contentType, "json") || (len(body) > 0 && body[0] == '{') {
		return nil, fmt.Errorf("amap returned non-image payload: %s", strings.TrimSpace(string(body)))
	}

	return body, nil
}

func buildAMapMarkers(positions []models.Position) string {
	if len(positions) == 0 {
		return ""
	}

	first := positions[0]
	last := positions[len(positions)-1]
	return fmt.Sprintf(
		"mid,0x2FBF71,A:%.6f,%.6f|mid,0xFC6054,B:%.6f,%.6f",
		first.Longitude, first.Latitude,
		last.Longitude, last.Latitude,
	)
}

func buildAMapPath(positions []models.Position) string {
	parts := make([]string, 0, len(positions))
	for _, p := range positions {
		parts = append(parts, fmt.Sprintf("%.6f,%.6f", p.Longitude, p.Latitude))
	}
	return "8,0x54c5ff,1,,:" + strings.Join(parts, ";")
}

func aMapUserAgent(cfg *config.Config) string {
	if ua := strings.TrimSpace(cfg.OSMTileUserAgent); ua != "" {
		return ua
	}
	return "tete-manager-notifier/1.0"
}
