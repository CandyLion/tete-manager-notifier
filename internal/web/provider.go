package web

import (
	"fmt"
	"strings"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
)

const (
	MapProviderOSM  = "osm"
	MapProviderAMap = "amap"
)

type mapProviderLink struct {
	ID     string
	Label  string
	URL    string
	Active bool
}

func normalizeProvider(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case MapProviderOSM, "openstreetmap":
		return MapProviderOSM
	case MapProviderAMap, "gaode", "autonavi":
		return MapProviderAMap
	default:
		return ""
	}
}

func providerLabel(provider string) string {
	switch provider {
	case MapProviderAMap:
		return "高德"
	default:
		return "OSM"
	}
}

func barkMapProvider(cfg *config.Config) string {
	return resolveTrackProvider(cfg, cfg.BarkMapProvider, MapProviderOSM)
}

func resolveTrackProvider(cfg *config.Config, requested string, fallback string) string {
	for _, candidate := range []string{requested, fallback, MapProviderOSM, MapProviderAMap} {
		provider := normalizeProvider(candidate)
		if provider == "" {
			continue
		}
		if isTrackProviderAvailable(cfg, provider) {
			return provider
		}
	}
	return ""
}

func resolveDetailProvider(cfg *config.Config, requested string) string {
	available := detailProviders(cfg)
	if len(available) == 0 {
		return MapProviderOSM
	}

	requested = normalizeProvider(requested)
	for _, provider := range available {
		if provider == requested {
			return provider
		}
	}

	defaultProvider := normalizeProvider(cfg.DetailMapDefaultProvider)
	for _, provider := range available {
		if provider == defaultProvider {
			return provider
		}
	}

	return available[0]
}

func detailProviders(cfg *config.Config) []string {
	if cfg == nil {
		return []string{MapProviderOSM}
	}

	providers := make([]string, 0, len(cfg.DetailMapProviders))
	seen := map[string]bool{}
	for _, raw := range cfg.DetailMapProviders {
		provider := normalizeProvider(raw)
		if provider == "" || seen[provider] || !isDetailProviderAvailable(cfg, provider) {
			continue
		}
		seen[provider] = true
		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		if isDetailProviderAvailable(cfg, MapProviderOSM) {
			return []string{MapProviderOSM}
		}
		if isDetailProviderAvailable(cfg, MapProviderAMap) {
			return []string{MapProviderAMap}
		}
	}

	return providers
}

func buildDetailProviderLinks(cfg *config.Config, driveID uint, current string) []mapProviderLink {
	providers := detailProviders(cfg)
	links := make([]mapProviderLink, 0, len(providers))
	for _, provider := range providers {
		links = append(links, mapProviderLink{
			ID:     provider,
			Label:  providerLabel(provider),
			URL:    BuildDriveDetailPathForProvider(cfg, driveID, provider),
			Active: provider == current,
		})
	}
	return links
}

func isDetailProviderAvailable(cfg *config.Config, provider string) bool {
	switch provider {
	case MapProviderAMap:
		return strings.TrimSpace(cfg.AMapJSKey) != ""
	case MapProviderOSM:
		return strings.TrimSpace(cfg.OSMTileURL) != ""
	default:
		return false
	}
}

func isTrackProviderAvailable(cfg *config.Config, provider string) bool {
	switch provider {
	case MapProviderAMap:
		return strings.TrimSpace(cfg.AMapStaticMapURL) != "" && strings.TrimSpace(cfg.AMapWebServiceKey) != ""
	case MapProviderOSM:
		return strings.TrimSpace(cfg.OSMTileURL) != ""
	default:
		return false
	}
}

func providerUnavailableHint(provider string) string {
	switch provider {
	case MapProviderAMap:
		return "高德地图当前未配置 AMAP_JS_KEY / AMAP_WEB_SERVICE_KEY"
	case MapProviderOSM:
		return "OSM 底图当前不可用"
	default:
		return fmt.Sprintf("地图提供方 %s 当前不可用", provider)
	}
}
