package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aishow/internal/models"
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 0}}
}

func SpeechURL(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	switch {
	case b == "":
		return ""
	case strings.HasSuffix(b, "/v1/audio/speech"), strings.HasSuffix(b, "/audio/speech"):
		return b
	case strings.HasSuffix(b, "/v1"):
		return b + "/audio/speech"
	default:
		return b + "/v1/audio/speech"
	}
}

func (c *Client) Speak(ctx context.Context, base, token, model, voice, text, dest string) error {
	url := SpeechURL(base)
	if url == "" {
		return fmt.Errorf("未配置本地 TTS 地址")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("没有对白")
	}
	payload := map[string]any{
		"model":           emptyAs(model, "tts-1"),
		"input":           text,
		"voice":           emptyAs(voice, "alloy"),
		"response_format": "wav",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("TTS 响应超时")
		}
		return fmt.Errorf("连接本地 TTS 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("本地 TTS 错误 HTTP %d: %s", resp.StatusCode, clip(raw, 200))
	}
	if len(raw) < 32 {
		return fmt.Errorf("TTS 没有返回音频")
	}
	if err := os.MkdirAll(parentDir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, raw, 0o644)
}

func (c *Client) Inspect(base, token string) models.SidecarHealth {
	start := time.Now()
	url := SpeechURL(base)
	if url == "" {
		return models.SidecarHealth{Detail: "未配置"}
	}
	root := strings.TrimSuffix(strings.TrimSuffix(url, "/audio/speech"), "/v1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(root, "/")+"/v1/models", nil)
	if err != nil {
		return models.SidecarHealth{Detail: "离线"}
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return models.SidecarHealth{LatencyMS: lat, Detail: "离线"}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 500 {
		return models.SidecarHealth{LatencyMS: lat, Detail: "离线"}
	}
	return models.SidecarHealth{Ready: true, LatencyMS: lat, Detail: "在线", TextEncoderLabel: "本地 TTS"}
}

func emptyAs(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func parentDir(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i <= 0 {
		return "."
	}
	return path[:i]
}
