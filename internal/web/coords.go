package web

import (
	"math"

	"github.com/CandyLion/tmbark-notifier/internal/models"
)

const (
	earthA  = 6378245.0
	earthEE = 0.00669342162296594323
)

func positionsForProvider(provider string, positions []models.Position) []models.Position {
	switch normalizeProvider(provider) {
	case MapProviderAMap:
		return convertPositionsToGCJ02(positions)
	default:
		return positions
	}
}

func convertPositionsToGCJ02(positions []models.Position) []models.Position {
	converted := make([]models.Position, 0, len(positions))
	for _, p := range positions {
		lat, lon := wgs84ToGCJ02(p.Latitude, p.Longitude)
		p.Latitude = lat
		p.Longitude = lon
		converted = append(converted, p)
	}
	return converted
}

func wgs84ToGCJ02(lat, lon float64) (float64, float64) {
	if outOfChina(lat, lon) {
		return lat, lon
	}

	dLat := transformLat(lon-105.0, lat-35.0)
	dLon := transformLon(lon-105.0, lat-35.0)
	radLat := lat / 180.0 * math.Pi
	sinLat := math.Sin(radLat)
	magic := 1 - earthEE*sinLat*sinLat
	sqrtMagic := math.Sqrt(magic)

	dLat = (dLat * 180.0) / ((earthA * (1 - earthEE)) / (magic * sqrtMagic) * math.Pi)
	dLon = (dLon * 180.0) / (earthA / sqrtMagic * math.Cos(radLat) * math.Pi)

	return lat + dLat, lon + dLon
}

func outOfChina(lat, lon float64) bool {
	if lon < 72.004 || lon > 137.8347 {
		return true
	}
	if lat < 0.8293 || lat > 55.8271 {
		return true
	}
	return false
}

func transformLat(x, y float64) float64 {
	ret := -100.0 + 2.0*x + 3.0*y + 0.2*y*y + 0.1*x*y + 0.2*math.Sqrt(math.Abs(x))
	ret += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(y*math.Pi) + 40.0*math.Sin(y/3.0*math.Pi)) * 2.0 / 3.0
	ret += (160.0*math.Sin(y/12.0*math.Pi) + 320*math.Sin(y*math.Pi/30.0)) * 2.0 / 3.0
	return ret
}

func transformLon(x, y float64) float64 {
	ret := 300.0 + x + 2.0*y + 0.1*x*x + 0.1*x*y + 0.1*math.Sqrt(math.Abs(x))
	ret += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(x*math.Pi) + 40.0*math.Sin(x/3.0*math.Pi)) * 2.0 / 3.0
	ret += (150.0*math.Sin(x/12.0*math.Pi) + 300.0*math.Sin(x/30.0*math.Pi)) * 2.0 / 3.0
	return ret
}
