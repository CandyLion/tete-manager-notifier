package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
)

func BuildDriveDetailURL(cfg *config.Config, driveID uint) string {
	return BuildDriveDetailURLForProvider(cfg, driveID, "")
}

func BuildDriveTrackImageURL(cfg *config.Config, driveID uint) string {
	return BuildDriveTrackImageURLForProvider(cfg, driveID, "")
}

func BuildDriveDetailPath(cfg *config.Config, driveID uint) string {
	return BuildDriveDetailPathForProvider(cfg, driveID, "")
}

func BuildDriveTrackImagePath(cfg *config.Config, driveID uint) string {
	return BuildDriveTrackImagePathForProvider(cfg, driveID, "")
}

func BuildDriveDetailURLForProvider(cfg *config.Config, driveID uint, provider string) string {
	values := url.Values{}
	if provider = normalizeProvider(provider); provider != "" {
		values.Set("map", provider)
	}
	return buildSignedPublicURL(cfg, fmt.Sprintf("/drives/%d", driveID), values)
}

func BuildDriveTrackImageURLForProvider(cfg *config.Config, driveID uint, provider string) string {
	values := url.Values{}
	if provider = normalizeProvider(provider); provider != "" {
		values.Set("provider", provider)
	}
	return buildSignedPublicURL(cfg, fmt.Sprintf("/drives/%d/track.png", driveID), values)
}

func BuildDriveDetailPathForProvider(cfg *config.Config, driveID uint, provider string) string {
	values := url.Values{}
	if provider = normalizeProvider(provider); provider != "" {
		values.Set("map", provider)
	}
	return buildSignedPath(cfg, fmt.Sprintf("/drives/%d", driveID), values)
}

func BuildDriveTrackImagePathForProvider(cfg *config.Config, driveID uint, provider string) string {
	values := url.Values{}
	if provider = normalizeProvider(provider); provider != "" {
		values.Set("provider", provider)
	}
	return buildSignedPath(cfg, fmt.Sprintf("/drives/%d/track.png", driveID), values)
}

func ValidateSignedRequest(cfg *config.Config, r *http.Request) bool {
	if strings.TrimSpace(cfg.WebURLSecret) == "" {
		return true
	}

	expected := signPath(cfg.WebURLSecret, r.URL.Path)
	actual := r.URL.Query().Get("sig")
	if actual == "" {
		return false
	}

	return hmac.Equal([]byte(expected), []byte(actual))
}

func buildSignedPublicURL(cfg *config.Config, requestPath string, extraQuery url.Values) string {
	base := strings.TrimSpace(cfg.PublicBaseURL)
	if base == "" {
		return ""
	}

	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	u := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   requestPath,
	}

	if len(extraQuery) > 0 {
		u.RawQuery = extraQuery.Encode()
	}

	if strings.TrimSpace(cfg.WebURLSecret) != "" {
		q := u.Query()
		q.Set("sig", signPath(cfg.WebURLSecret, requestPath))
		u.RawQuery = q.Encode()
	}

	return u.String()
}

func buildSignedPath(cfg *config.Config, requestPath string, extraQuery url.Values) string {
	values := url.Values{}
	for key, items := range extraQuery {
		for _, item := range items {
			values.Add(key, item)
		}
	}

	if strings.TrimSpace(cfg.WebURLSecret) != "" {
		values.Set("sig", signPath(cfg.WebURLSecret, requestPath))
	}
	if len(values) == 0 {
		return requestPath
	}
	return requestPath + "?" + values.Encode()
}

func signPath(secret, requestPath string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(requestPath))
	return hex.EncodeToString(mac.Sum(nil))
}
