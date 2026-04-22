package notifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/wen-ryon/tete-manager-notifier/internal/config"
)

type Payload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Options struct {
	ClickURL string
	ImageURL string
}

type barkPayload struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Group string `json:"group,omitempty"`
	Sound string `json:"sound,omitempty"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Image string `json:"image,omitempty"`
	Level string `json:"level,omitempty"`
}

// 保留原有特特管家推送
func SendNotification(apiEndpoint, title, content string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("推送通知 panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	apiEndpoint = strings.TrimSpace(apiEndpoint)
	if apiEndpoint == "" {
		return nil
	}

	payload := Payload{
		Title:   title,
		Content: content,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tete status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// 双发入口：特特管家 + Bark
func SendAll(cfg *config.Config, title, content string, options ...Options) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("推送通知 panic: %v\n%s", r, string(debug.Stack()))
		}
	}()

	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	var opt Options
	if len(options) > 0 {
		opt = options[0]
	}

	var errs []string

	if strings.TrimSpace(cfg.APIToken) != "" {
		if err := SendNotification(cfg.APIToken, title, content); err != nil {
			errs = append(errs, "tete push failed: "+err.Error())
		}
	}

	if strings.TrimSpace(cfg.BarkKey) != "" {
		if err := sendBark(cfg, title, content, opt); err != nil {
			errs = append(errs, "bark push failed: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func sendBark(cfg *config.Config, title, content string, opt Options) error {
	base := strings.TrimRight(strings.TrimSpace(cfg.BarkServer), "/")
	if base == "" {
		base = "https://api.day.app"
	}

	key := strings.TrimSpace(cfg.BarkKey)
	if key == "" {
		return nil
	}

	pushURL := fmt.Sprintf("%s/%s", base, neturl.PathEscape(key))

	urlValue := strings.TrimSpace(opt.ClickURL)
	if urlValue == "" {
		urlValue = strings.TrimSpace(cfg.BarkURL)
	}

	payload := barkPayload{
		Title: title,
		Body:  content,
		Group: strings.TrimSpace(cfg.BarkGroup),
		Sound: strings.TrimSpace(cfg.BarkSound),
		URL:   urlValue,
		Icon:  strings.TrimSpace(cfg.BarkIcon),
		Image: strings.TrimSpace(opt.ImageURL),
		Level: strings.TrimSpace(cfg.BarkLevel),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, pushURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("bark status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}
