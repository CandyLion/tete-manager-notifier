package notifier

import (
"bytes"
"encoding/json"
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
func SendAll(cfg *config.Config, title, content string) error {
defer func() {
if r := recover(); r != nil {
log.Printf("推送通知 panic: %v\n%s", r, string(debug.Stack()))
}
}()

if cfg == nil {
return fmt.Errorf("config is nil")
}

var errs []string

if strings.TrimSpace(cfg.APIToken) != "" {
if err := SendNotification(cfg.APIToken, title, content); err != nil {
errs = append(errs, "tete push failed: "+err.Error())
}
}

if strings.TrimSpace(cfg.BarkKey) != "" {
if err := sendBark(cfg, title, content); err != nil {
errs = append(errs, "bark push failed: "+err.Error())
}
}

if len(errs) > 0 {
return fmt.Errorf(strings.Join(errs, "; "))
}
return nil
}

func sendBark(cfg *config.Config, title, content string) error {
base := strings.TrimRight(strings.TrimSpace(cfg.BarkServer), "/")
if base == "" {
base = "https://api.day.app"
}

key := strings.TrimSpace(cfg.BarkKey)
if key == "" {
return nil
}

pushURL := fmt.Sprintf(
"%s/%s/%s/%s",
base,
neturl.PathEscape(key),
neturl.PathEscape(title),
neturl.PathEscape(content),
)

q := neturl.Values{}
if v := strings.TrimSpace(cfg.BarkGroup); v != "" {
q.Set("group", v)
}
if v := strings.TrimSpace(cfg.BarkSound); v != "" {
q.Set("sound", v)
}
if v := strings.TrimSpace(cfg.BarkURL); v != "" {
q.Set("url", v)
}
if v := strings.TrimSpace(cfg.BarkIcon); v != "" {
q.Set("icon", v)
}
if v := strings.TrimSpace(cfg.BarkLevel); v != "" {
q.Set("level", v)
}
if encoded := q.Encode(); encoded != "" {
pushURL += "?" + encoded
}

client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get(pushURL)
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