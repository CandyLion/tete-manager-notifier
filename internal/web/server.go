package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
	"github.com/wen-ryon/tete-manager-notifier/internal/db"
	"github.com/wen-ryon/tete-manager-notifier/internal/models"

	"gorm.io/gorm"
)

type Server struct {
	cfg *config.Config
	srv *http.Server
}

type drivePageData struct {
	CarName          string
	DriveID          uint
	TripDate         string
	StartTime        string
	EndTime          string
	StartClock       string
	EndClock         string
	Duration         string
	Distance         string
	AvgSpeed         string
	Battery          string
	BatteryHint      string
	Range            string
	StartCoord       string
	EndCoord         string
	Efficiency       string
	EfficiencyHint   string
	RouteOverview    string
	RouteHint        string
	TrackSampling    string
	TrackImageURL    string
	DownloadTrackURL string
	DownloadTrackCTA string
	GeneratedAt      string
	MapPointsJSON    template.JS
	MapProvider      string
	MapProviderLabel string
	MapProviders     []mapProviderLink
	UseLeaflet       bool
	UseAMap          bool
	AMapLoaderURL    string
	AMapSecurityCode string
}

var driveDetailTemplate = template.Must(template.New("drive-detail").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .CarName }} 行程详情</title>
  {{ if .UseLeaflet }}
  <link
    rel="stylesheet"
    href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
    integrity="sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY="
    crossorigin=""
  >
  {{ end }}
  {{ if .UseAMap }}
  {{ if .AMapSecurityCode }}
  <script>
    window._AMapSecurityConfig = {
      securityJsCode: {{ printf "%q" .AMapSecurityCode }}
    };
  </script>
  {{ end }}
  <script src="{{ .AMapLoaderURL }}"></script>
  {{ end }}
  <style>
    :root {
      --bg-top: #081627;
      --bg-bottom: #14405b;
      --card: rgba(255,255,255,0.08);
      --card-border: rgba(255,255,255,0.14);
      --text: #eef7ff;
      --muted: rgba(238,247,255,0.7);
      --accent: #54c5ff;
      --accent-soft: rgba(84,197,255,0.16);
      --success: #6ce2a2;
      --danger: #ff916d;
      --shadow: 0 24px 60px rgba(0,0,0,0.28);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      color: var(--text);
      font-family: "Avenir Next", "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
      background:
        radial-gradient(circle at top left, rgba(84,197,255,0.18), transparent 36%),
        linear-gradient(180deg, var(--bg-top), var(--bg-bottom));
    }
    .page {
      width: min(980px, calc(100vw - 32px));
      margin: 0 auto;
      padding: 28px 0 48px;
    }
    .hero {
      display: grid;
      gap: 18px;
      margin-bottom: 20px;
    }
    .hero-card, .meta-card, .footnote {
      background: var(--card);
      border: 1px solid var(--card-border);
      border-radius: 24px;
      box-shadow: var(--shadow);
      backdrop-filter: blur(10px);
    }
    .hero-card {
      padding: 24px;
      overflow: hidden;
    }
    .eyebrow {
      margin: 0 0 10px;
      font-size: 13px;
      letter-spacing: 0.16em;
      text-transform: uppercase;
      color: var(--muted);
    }
    h1 {
      margin: 0;
      font-size: clamp(30px, 5vw, 52px);
      line-height: 0.96;
      font-weight: 700;
    }
    .subline {
      margin-top: 12px;
      color: var(--muted);
      font-size: 15px;
      line-height: 1.5;
    }
    .subline-piece {
      display: block;
    }
    .subline-piece--inline {
      display: inline;
    }
    .track-frame {
      margin-top: 20px;
      padding: 12px;
      border-radius: 22px;
      background: rgba(255,255,255,0.06);
      border: 1px solid rgba(255,255,255,0.08);
    }
    .map-toolbar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      margin-top: 18px;
    }
    .provider-switch {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 6px;
      border-radius: 999px;
      background: rgba(255,255,255,0.06);
      border: 1px solid rgba(255,255,255,0.09);
    }
    .provider-chip {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 68px;
      min-height: 34px;
      padding: 0 14px;
      border-radius: 999px;
      color: var(--muted);
      text-decoration: none;
      font-size: 13px;
      font-weight: 600;
      transition: background 0.18s ease, color 0.18s ease;
    }
    .provider-chip.is-active {
      background: rgba(84,197,255,0.18);
      color: var(--text);
      box-shadow: inset 0 0 0 1px rgba(84,197,255,0.22);
    }
    .provider-chip:not(.is-active):hover {
      background: rgba(255,255,255,0.08);
      color: var(--text);
    }
    #route-map {
      display: block;
      width: 100%;
      min-height: 420px;
      border-radius: 18px;
      background: #0a1523;
    }
    .amap-dot {
      width: 24px;
      height: 24px;
      border-radius: 999px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: #fff;
      font-size: 12px;
      font-weight: 700;
      border: 2px solid rgba(255,255,255,0.9);
      box-shadow: 0 8px 18px rgba(0,0,0,0.22);
      background: #54c5ff;
    }
    .amap-dot--start {
      background: #2fbf71;
    }
    .amap-dot--end {
      background: #fc6054;
    }
    .map-fallback {
      min-height: 420px;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
      color: var(--muted);
      text-align: center;
      line-height: 1.6;
    }
    .leaflet-container {
      font: inherit;
      background: #0a1523;
    }
    .leaflet-control-attribution {
      background: rgba(8, 22, 39, 0.86);
      color: rgba(238,247,255,0.82);
    }
    .leaflet-control-attribution a {
      color: var(--accent);
    }
    .leaflet-popup-content-wrapper,
    .leaflet-popup-tip {
      background: #10263b;
      color: var(--text);
    }
    .legend {
      display: flex;
      flex-wrap: wrap;
      gap: 10px 14px;
      margin-top: 14px;
      color: var(--muted);
      font-size: 13px;
    }
    .legend-item {
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }
    .legend-dot {
      width: 12px;
      height: 12px;
      border-radius: 999px;
      box-shadow: 0 0 0 4px rgba(255,255,255,0.08);
    }
    .legend-start { background: var(--success); }
    .legend-end { background: var(--danger); }
    .action-row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 12px;
      margin-top: 16px;
    }
    .button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-height: 42px;
      padding: 0 16px;
      border-radius: 999px;
      border: 1px solid rgba(255,255,255,0.12);
      background: rgba(255,255,255,0.08);
      color: var(--text);
      text-decoration: none;
      font-size: 14px;
      font-weight: 600;
    }
    .button:hover {
      background: rgba(255,255,255,0.12);
    }
    .meta-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }
    .meta-card {
      padding: 14px 14px 12px;
      min-height: 108px;
      border-radius: 18px;
      background: rgba(255,255,255,0.07);
      border-color: rgba(255,255,255,0.1);
    }
    .meta-label {
      color: var(--muted);
      font-size: 11px;
      margin-bottom: 6px;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }
    .meta-value {
      font-size: clamp(20px, 2.8vw, 28px);
      font-weight: 700;
      line-height: 1.08;
    }
    .meta-value--compact {
      font-size: clamp(16px, 2.2vw, 22px);
      line-height: 1.24;
    }
    .meta-hint {
      margin-top: 6px;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.45;
    }
    .footnote {
      margin-top: 12px;
      padding: 14px 16px;
      color: var(--muted);
      font-size: 12.5px;
      line-height: 1.55;
    }
    .footnote strong {
      color: var(--text);
      font-weight: 600;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      padding: 8px 12px;
      border-radius: 999px;
      background: var(--accent-soft);
      color: var(--text);
      font-size: 13px;
      margin-top: 16px;
    }
    @media (max-width: 720px) {
      .page {
        width: min(100vw - 20px, 980px);
        padding-top: 16px;
      }
      .hero-card {
        padding: 18px;
      }
      .map-toolbar {
        align-items: flex-start;
      }
      .meta-grid {
        gap: 8px;
      }
      .meta-card {
        min-height: 92px;
        padding: 12px 12px 11px;
      }
      .meta-value {
        font-size: 20px;
      }
      .meta-value--compact {
        font-size: 15px;
      }
    }
    @media (max-width: 460px) {
      .page {
        width: min(100vw - 14px, 980px);
      }
      .track-frame {
        padding: 8px;
      }
      .provider-switch {
        width: 100%;
        justify-content: stretch;
      }
      .provider-chip {
        flex: 1 1 0;
      }
      .meta-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 7px;
      }
      .meta-card {
        min-height: 84px;
        padding: 10px 10px 9px;
      }
      .meta-label {
        font-size: 10px;
        margin-bottom: 4px;
      }
      .meta-value {
        font-size: 17px;
      }
      .meta-value--compact {
        font-size: 13px;
        line-height: 1.18;
      }
      .meta-hint {
        margin-top: 4px;
        font-size: 10.5px;
        line-height: 1.35;
      }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="hero">
      <article class="hero-card">
        <p class="eyebrow">Trip Detail</p>
        <h1>{{ .CarName }} 行程详情</h1>
        <div class="subline">
          <span class="subline-piece subline-piece--inline">{{ .TripDate }} · 行程 #{{ .DriveID }}</span>
          <span class="subline-piece">{{ .StartTime }} → {{ .EndTime }}</span>
        </div>
        <div class="map-toolbar">
          <div class="provider-switch">
            {{ range .MapProviders }}
            <a class="provider-chip{{ if .Active }} is-active{{ end }}" href="{{ .URL }}">{{ .Label }}</a>
            {{ end }}
          </div>
          <div class="subline">当前地图：{{ .MapProviderLabel }}</div>
        </div>
        <div class="pill">{{ .TrackSampling }}</div>
        <div class="track-frame">
          <div id="route-map">
            <div class="map-fallback">地图加载中...</div>
          </div>
        </div>
        <div class="legend">
          <span class="legend-item"><span class="legend-dot legend-start"></span> 起点</span>
          <span class="legend-item"><span class="legend-dot legend-end"></span> 终点</span>
          <span class="legend-item">轨迹图基于真实位置点绘制</span>
        </div>
        <div class="action-row">
          <a class="button" href="{{ .DownloadTrackURL }}" target="_blank" rel="noreferrer">{{ .DownloadTrackCTA }}</a>
          <span class="subline">页面生成于 {{ .GeneratedAt }}</span>
        </div>
      </article>
    </section>

    <section class="meta-grid">
      <article class="meta-card">
        <div class="meta-label">起点</div>
        <div class="meta-value">{{ .StartClock }}</div>
        <div class="meta-hint">{{ .StartCoord }}</div>
      </article>
      <article class="meta-card">
        <div class="meta-label">终点</div>
        <div class="meta-value">{{ .EndClock }}</div>
        <div class="meta-hint">{{ .EndCoord }}</div>
      </article>
      <article class="meta-card">
        <div class="meta-label">时长</div>
        <div class="meta-value">{{ .Duration }}</div>
        <div class="meta-hint">平均速度 {{ .AvgSpeed }}</div>
      </article>
      <article class="meta-card">
        <div class="meta-label">距离</div>
        <div class="meta-value">{{ .Distance }}</div>
        <div class="meta-hint">表显变化 {{ .Range }}</div>
      </article>
      <article class="meta-card">
        <div class="meta-label">电量</div>
        <div class="meta-value meta-value--compact">{{ .Battery }}</div>
        <div class="meta-hint">{{ .BatteryHint }}</div>
      </article>
      <article class="meta-card">
        <div class="meta-label">路线</div>
        <div class="meta-value meta-value--compact">{{ .RouteOverview }}</div>
        <div class="meta-hint">{{ .RouteHint }}</div>
      </article>
    </section>

    <section class="footnote">
      <strong>说明：</strong> 这张轨迹图根据 TeslaMate 数据库中的真实位置点绘制，不做高德重规划。
      图片渲染时会按 <code>TRACK_MAX_POINTS</code> 对轨迹点抽样，避免超长行程导致图像过重，但详情统计仍按整段行程数据展示。
      如果某次行程缺少轨迹点，页面会回退为起终点连线示意。
      如果公网访问，建议为 <code>PUBLIC_BASE_URL</code> 配置 HTTPS，并同时设置 <code>WEB_URL_SECRET</code> 保护详情页链接。
    </section>
  </main>
  {{ if .UseLeaflet }}
  <script
    src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"
    integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="
    crossorigin=""
  ></script>
  {{ end }}
  <script>
    (() => {
      const trackPoints = {{ .MapPointsJSON }};
      const mapEl = document.getElementById('route-map');
      if (!mapEl) {
        return;
      }

      const fallback = (message) => {
        mapEl.innerHTML = '<div class="map-fallback">' + message + '</div>';
      };

      if (!Array.isArray(trackPoints) || trackPoints.length === 0) {
        fallback('这次行程没有可用的坐标点，暂时只能查看下方统计信息。');
        return;
      }

      {{ if .UseLeaflet }}
      if (typeof window.L === 'undefined') {
        fallback('地图底图加载失败，但你仍然可以使用“打开轨迹原图”查看线路。');
        return;
      }

      const map = L.map('route-map', {
        zoomControl: true,
        scrollWheelZoom: true,
      });

      L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
      }).addTo(map);

      const routeLine = L.polyline(trackPoints, {
        color: '#54c5ff',
        weight: 5,
        opacity: 0.92,
        lineJoin: 'round'
      }).addTo(map);

      const start = trackPoints[0];
      const end = trackPoints[trackPoints.length - 1];

      L.circleMarker(start, {
        radius: 7,
        color: '#ffffff',
        weight: 2,
        fillColor: '#6ce2a2',
        fillOpacity: 1
      }).addTo(map).bindPopup('起点');

      L.circleMarker(end, {
        radius: 7,
        color: '#ffffff',
        weight: 2,
        fillColor: '#ff916d',
        fillOpacity: 1
      }).addTo(map).bindPopup('终点');

      map.fitBounds(routeLine.getBounds(), {
        padding: [28, 28]
      });

      requestAnimationFrame(() => {
        map.invalidateSize();
      });
      {{ end }}

      {{ if .UseAMap }}
      if (typeof window.AMap === 'undefined') {
        fallback('高德地图脚本加载失败，你仍然可以使用“打开轨迹原图”查看线路。');
        return;
      }

      const map = new window.AMap.Map('route-map', {
        resizeEnable: true,
        zoom: 12,
        center: trackPoints[0],
        mapStyle: 'amap://styles/normal'
      });

      const polyline = new window.AMap.Polyline({
        path: trackPoints,
        strokeColor: '#54c5ff',
        strokeOpacity: 0.95,
        strokeWeight: 6,
        borderWeight: 2,
        outlineColor: '#183649'
      });

      const startMarker = new window.AMap.Marker({
        position: trackPoints[0],
        content: '<div class="amap-dot amap-dot--start">起</div>',
        offset: new window.AMap.Pixel(-12, -12)
      });

      const endMarker = new window.AMap.Marker({
        position: trackPoints[trackPoints.length - 1],
        content: '<div class="amap-dot amap-dot--end">终</div>',
        offset: new window.AMap.Pixel(-12, -12)
      });

      map.add([polyline, startMarker, endMarker]);
      map.setFitView([polyline, startMarker, endMarker], false, [28, 28, 28, 28]);
      {{ end }}
    })();
  </script>
</body>
</html>`))

func NewServer(cfg *config.Config) *Server {
	mux := http.NewServeMux()
	s := &Server{cfg: cfg}
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/drives/", s.handleDrive)

	s.srv = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.WebHost, cfg.WebPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) Addr() string {
	if s == nil || s.srv == nil {
		return ""
	}
	return s.srv.Addr
}

func (s *Server) ListenAndServe() error {
	if s == nil || s.srv == nil || s.cfg.WebPort <= 0 {
		return nil
	}
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil || s.cfg.WebPort <= 0 {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleDrive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !ValidateSignedRequest(s.cfg, r) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/drives/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	driveID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch {
	case len(parts) == 1:
		s.serveDriveDetail(w, r, uint(driveID))
	case len(parts) == 2 && parts[1] == "track.png":
		s.serveDriveTrack(w, r, uint(driveID))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveDriveDetail(w http.ResponseWriter, r *http.Request, driveID uint) {
	detail, positions, err := s.loadDriveData(driveID)
	if err != nil {
		if errors.Is(err, errDriveNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("❌ 行程详情加载失败: %v", err)
		http.Error(w, "failed to load drive", http.StatusInternalServerError)
		return
	}

	mapProvider := resolveDetailProvider(s.cfg, r.URL.Query().Get("map"))
	trackProvider := resolveTrackProvider(s.cfg, mapProvider, barkMapProvider(s.cfg))
	carName, err := db.GetCarName(s.cfg.CarID)
	if err != nil || carName == "" {
		carName = "我的 Tesla"
	}

	sampledPositions := SamplePositions(positions, s.cfg.TrackMaxPoints)
	mapPositions := SamplePositions(positions, mapPointLimit(s.cfg.TrackMaxPoints))
	socUsed := detail.StartSOC - detail.EndSOC
	rangeReduced := detail.StartIdealRangeKM - detail.EndIdealRangeKM
	kmPerPercent := calculateKmPerPercent(detail.Distance, socUsed)
	rangeAchievement := calculateRangeAchievement(detail.Distance, rangeReduced)
	directDistance := calculateDirectDistanceKm(detail.StartLatitude, detail.StartLongitude, detail.EndLatitude, detail.EndLongitude)
	routeFactor := calculateRouteFactor(detail.Distance, directDistance)
	generatedAt := time.Now().Local().Format("2006-01-02 15:04:05")

	efficiency := "暂无数据"
	if kmPerPercent > 0 {
		efficiency = fmt.Sprintf("%.2f km/%%", kmPerPercent)
	}

	efficiencyHint := "表显达成率 暂无数据"
	if rangeAchievement > 0 {
		efficiencyHint = fmt.Sprintf("表显达成率 %.1f%% · 电量变化 %.0f%% → %.0f%% (%+.0f%%)",
			rangeAchievement, detail.StartSOC, detail.EndSOC, detail.EndSOC-detail.StartSOC)
	} else {
		efficiencyHint = fmt.Sprintf("电量变化 %.0f%% → %.0f%% (%+.0f%%)",
			detail.StartSOC, detail.EndSOC, detail.EndSOC-detail.StartSOC)
	}

	trackSampling := fmt.Sprintf("%d 轨迹点", len(positions))
	if len(sampledPositions) != len(positions) {
		trackSampling = fmt.Sprintf("%d 原始点 · %d 绘图点", len(positions), len(sampledPositions))
	}

	routeOverview := trackSampling
	routeHint := "页面生成于 " + generatedAt
	if directDistance > 0 {
		routeOverview = fmt.Sprintf("直线 %.1f km", directDistance)
		if routeFactor > 0 {
			routeHint = fmt.Sprintf("绕行系数 %.2fx · %s", routeFactor, trackSampling)
		} else {
			routeHint = trackSampling
		}
	}

	data := drivePageData{
		CarName:          carName,
		DriveID:          driveID,
		TripDate:         detail.StartDate.Local().Format("2006-01-02"),
		StartTime:        detail.StartDate.Local().Format("2006-01-02 15:04"),
		EndTime:          detail.EndDate.Local().Format("2006-01-02 15:04"),
		StartClock:       detail.StartDate.Local().Format("15:04"),
		EndClock:         detail.EndDate.Local().Format("15:04"),
		Duration:         formatDuration(detail.DurationMin),
		Distance:         fmt.Sprintf("%.1f km", detail.Distance),
		AvgSpeed:         fmt.Sprintf("%.1f km/h", calculateAvgSpeed(detail.Distance, detail.DurationMin)),
		Battery:          fmt.Sprintf("%.0f%% → %.0f%% (%+.0f%%)", detail.StartSOC, detail.EndSOC, detail.EndSOC-detail.StartSOC),
		BatteryHint:      fmt.Sprintf("%s · %s", efficiency, efficiencyHint),
		Range:            fmt.Sprintf("%.0f km → %.0f km (%+.1f km)", detail.StartIdealRangeKM, detail.EndIdealRangeKM, detail.EndIdealRangeKM-detail.StartIdealRangeKM),
		StartCoord:       formatCoord(detail.StartLatitude, detail.StartLongitude),
		EndCoord:         formatCoord(detail.EndLatitude, detail.EndLongitude),
		Efficiency:       efficiency,
		EfficiencyHint:   efficiencyHint,
		RouteOverview:    routeOverview,
		RouteHint:        routeHint,
		TrackSampling:    trackSampling,
		TrackImageURL:    BuildDriveTrackImagePathForProvider(s.cfg, driveID, trackProvider),
		DownloadTrackURL: BuildDriveTrackImagePathForProvider(s.cfg, driveID, trackProvider),
		DownloadTrackCTA: fmt.Sprintf("打开%s轨迹原图", providerLabel(trackProvider)),
		GeneratedAt:      generatedAt,
		MapPointsJSON:    buildMapPointsJSON(mapProvider, mapPositions),
		MapProvider:      mapProvider,
		MapProviderLabel: providerLabel(mapProvider),
		MapProviders:     buildDetailProviderLinks(s.cfg, driveID, mapProvider),
		UseLeaflet:       mapProvider == MapProviderOSM,
		UseAMap:          mapProvider == MapProviderAMap,
		AMapLoaderURL:    buildAMapLoaderURL(s.cfg),
		AMapSecurityCode: s.cfg.AMapJSSecurityCode,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := driveDetailTemplate.Execute(w, data); err != nil {
		log.Printf("❌ 行程详情渲染失败: %v", err)
	}
}

func (s *Server) serveDriveTrack(w http.ResponseWriter, r *http.Request, driveID uint) {
	_, positions, err := s.loadDriveData(driveID)
	if err != nil {
		if errors.Is(err, errDriveNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Printf("❌ 轨迹图加载失败: %v", err)
		http.Error(w, "failed to load track", http.StatusInternalServerError)
		return
	}

	provider := resolveTrackProvider(s.cfg, r.URL.Query().Get("provider"), barkMapProvider(s.cfg))
	img, err := RenderTrackPNGForProvider(s.cfg, provider, SamplePositions(positions, s.cfg.TrackMaxPoints))
	if err != nil {
		log.Printf("❌ 轨迹图生成失败: %v", err)
		http.Error(w, "failed to render track", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(img)
}

var errDriveNotFound = errors.New("drive not found")

func (s *Server) loadDriveData(driveID uint) (*db.DriveDetail, []models.Position, error) {
	detail, err := db.GetDriveDetail(s.cfg.CarID, driveID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errDriveNotFound
		}
		return nil, nil, err
	}

	positions, err := db.GetDrivePositions(s.cfg.CarID, driveID, detail.StartDate, detail.EndDate)
	if err != nil {
		return nil, nil, err
	}

	positions = FilterTrackPositions(positions)
	if len(positions) == 0 {
		positions = fallbackTrack(detail)
	}

	return detail, positions, nil
}

func fallbackTrack(detail *db.DriveDetail) []models.Position {
	positions := make([]models.Position, 0, 2)
	if detail.StartLatitude != 0 || detail.StartLongitude != 0 {
		positions = append(positions, models.Position{
			Date:      detail.StartDate,
			Latitude:  detail.StartLatitude,
			Longitude: detail.StartLongitude,
		})
	}
	if detail.EndLatitude != 0 || detail.EndLongitude != 0 {
		positions = append(positions, models.Position{
			Date:      detail.EndDate,
			Latitude:  detail.EndLatitude,
			Longitude: detail.EndLongitude,
		})
	}
	return positions
}

func formatCoord(lat, lon float64) string {
	if lat == 0 && lon == 0 {
		return "暂无坐标"
	}
	return fmt.Sprintf("%.5f, %.5f", lat, lon)
}

func formatDuration(minutes int16) string {
	if minutes < 60 {
		return fmt.Sprintf("%d 分钟", minutes)
	}

	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		return fmt.Sprintf("%d 小时", hours)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", hours, mins)
}

func calculateAvgSpeed(distanceKm float64, durationMin int16) float64 {
	if durationMin == 0 {
		return 0
	}
	return distanceKm * 60 / float64(durationMin)
}

func calculateKmPerPercent(distanceKm, socUsed float64) float64 {
	if socUsed <= 0 {
		return 0
	}
	return distanceKm / socUsed
}

func calculateRangeAchievement(distanceKm, rangeReducedKm float64) float64 {
	if rangeReducedKm <= 0 {
		return 0
	}
	return (distanceKm / rangeReducedKm) * 100
}

func calculateRouteFactor(distanceKm, directDistanceKm float64) float64 {
	if directDistanceKm <= 0 {
		return 0
	}
	return distanceKm / directDistanceKm
}

func calculateDirectDistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	if (lat1 == 0 && lon1 == 0) || (lat2 == 0 && lon2 == 0) {
		return 0
	}

	const earthRadiusKm = 6371.0088
	toRad := func(deg float64) float64 {
		return deg * math.Pi / 180
	}

	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	lat1Rad := toRad(lat1)
	lat2Rad := toRad(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func buildMapPointsJSON(provider string, positions []models.Position) template.JS {
	providerPositions := positionsForProvider(provider, positions)
	points := make([][2]float64, 0, len(providerPositions))
	for _, p := range providerPositions {
		if normalizeProvider(provider) == MapProviderAMap {
			points = append(points, [2]float64{p.Longitude, p.Latitude})
			continue
		}
		points = append(points, [2]float64{p.Latitude, p.Longitude})
	}

	data, err := json.Marshal(points)
	if err != nil {
		return template.JS("[]")
	}

	return template.JS(data)
}

func mapPointLimit(base int) int {
	if base <= 0 {
		return 480
	}

	limit := base * 4
	if limit < 300 {
		limit = 300
	}
	if limit > 1200 {
		limit = 1200
	}
	return limit
}

func buildAMapLoaderURL(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.AMapJSKey) == "" {
		return ""
	}
	return fmt.Sprintf(
		"https://webapi.amap.com/maps?v=%s&key=%s",
		strings.TrimSpace(cfg.AMapJSVersion),
		strings.TrimSpace(cfg.AMapJSKey),
	)
}
