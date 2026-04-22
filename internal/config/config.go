package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	APIToken string

	BarkServer string
	BarkKey    string
	BarkGroup  string
	BarkSound  string
	BarkURL    string
	BarkIcon   string
	BarkLevel  string

	DBHost string
	DBUser string
	DBPass string
	DBName string
	DBPort int

	MQTTHost string
	MQTTPort int
	MQTTUser string
	MQTTPass string

	CarID int

	WebHost        string
	WebPort        int
	PublicBaseURL  string
	WebURLSecret   string
	TrackMaxPoints int

	BarkMapProvider          string
	DetailMapDefaultProvider string
	DetailMapProviders       []string

	OSMTileURL        string
	OSMTileUserAgent  string
	OSMTileCacheHours int

	AMapStaticMapURL   string
	AMapWebServiceKey  string
	AMapJSKey          string
	AMapJSSecurityCode string
	AMapJSVersion      string

	LogLevel        string
	PushDebounceSec int // 推送防抖初始时间，后续会进行3次指数退避重试，按(次数-1)倍增加
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		APIToken: os.Getenv("API_TOKEN"),

		BarkServer: getEnv("BARK_SERVER", "https://api.day.app"),
		BarkKey:    os.Getenv("BARK_KEY"),
		BarkGroup:  os.Getenv("BARK_GROUP"),
		BarkSound:  os.Getenv("BARK_SOUND"),
		BarkURL:    os.Getenv("BARK_URL"),
		BarkIcon:   os.Getenv("BARK_ICON"),
		BarkLevel:  os.Getenv("BARK_LEVEL"),

		DBHost: getEnv("DATABASE_HOST", "database"),
		DBUser: getEnv("DATABASE_USER", "teslamate"),
		DBPass: os.Getenv("DATABASE_PASS"),
		DBName: getEnv("DATABASE_NAME", "teslamate"),
		DBPort: mustInt(os.Getenv("DATABASE_PORT"), 5432),

		MQTTHost: getEnv("MQTT_HOST", "mosquitto"),
		MQTTPort: mustInt(os.Getenv("MQTT_PORT"), 1883),
		MQTTUser: os.Getenv("MQTT_USER"),
		MQTTPass: os.Getenv("MQTT_PASS"),

		CarID: mustInt(os.Getenv("CAR_ID"), 1),

		WebHost:        getEnv("WEB_HOST", "0.0.0.0"),
		WebPort:        mustInt(os.Getenv("WEB_PORT"), 8080),
		PublicBaseURL:  trimTrailingSlash(os.Getenv("PUBLIC_BASE_URL")),
		WebURLSecret:   os.Getenv("WEB_URL_SECRET"),
		TrackMaxPoints: mustInt(os.Getenv("TRACK_MAX_POINTS"), 180),

		BarkMapProvider:          normalizeMapProvider(getEnv("BARK_MAP_PROVIDER", "osm"), "osm"),
		DetailMapDefaultProvider: normalizeMapProvider(getEnv("DETAIL_MAP_DEFAULT_PROVIDER", "osm"), "osm"),
		DetailMapProviders:       parseProviders(getEnv("DETAIL_MAP_ENABLED_PROVIDERS", "osm,amap")),

		OSMTileURL:        getEnv("OSM_TILE_URL", getEnv("MAP_TILE_URL", "https://tile.openstreetmap.org/{z}/{x}/{y}.png")),
		OSMTileUserAgent:  getEnv("OSM_TILE_USER_AGENT", getEnv("MAP_TILE_USER_AGENT", "tete-manager-notifier/1.0 (+https://github.com/wen-ryon/tete-manager-notifier)")),
		OSMTileCacheHours: mustInt(getEnv("OSM_TILE_CACHE_HOURS", getEnv("MAP_TILE_CACHE_HOURS", "168")), 168),

		AMapStaticMapURL:   getEnv("AMAP_STATIC_MAP_URL", "https://restapi.amap.com/v3/staticmap"),
		AMapWebServiceKey:  os.Getenv("AMAP_WEB_SERVICE_KEY"),
		AMapJSKey:          os.Getenv("AMAP_JS_KEY"),
		AMapJSSecurityCode: os.Getenv("AMAP_JS_SECURITY_CODE"),
		AMapJSVersion:      getEnv("AMAP_JS_VERSION", "2.0"),

		LogLevel:        getEnv("LOG_LEVEL", "info"),
		PushDebounceSec: mustInt(os.Getenv("PUSH_DEBOUNCE_SECONDS"), 5),
	}
}

func mustInt(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func trimTrailingSlash(v string) string {
	for len(v) > 0 && v[len(v)-1] == '/' {
		v = v[:len(v)-1]
	}
	return v
}

func parseProviders(raw string) []string {
	parts := splitCSV(raw)
	if len(parts) == 0 {
		return []string{"osm", "amap"}
	}

	providers := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		provider := normalizeMapProvider(part, "")
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		return []string{"osm", "amap"}
	}
	return providers
}

func splitCSV(raw string) []string {
	values := []string{}
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] != ',' {
			continue
		}

		part := raw[start:i]
		start = i + 1
		part = trimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func trimSpace(v string) string {
	start := 0
	for start < len(v) && isSpace(v[start]) {
		start++
	}
	end := len(v)
	for end > start && isSpace(v[end-1]) {
		end--
	}
	return v[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func normalizeMapProvider(v string, def string) string {
	switch trimSpace(lowerASCII(v)) {
	case "osm", "openstreetmap":
		return "osm"
	case "amap", "gaode", "autonavi":
		return "amap"
	case "":
		return def
	default:
		return def
	}
}

func lowerASCII(v string) string {
	buf := []byte(v)
	for i, b := range buf {
		if b >= 'A' && b <= 'Z' {
			buf[i] = b + ('a' - 'A')
		}
	}
	return string(buf)
}
