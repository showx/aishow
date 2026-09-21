package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aishow/internal/models"
)

type Client struct {
	http *http.Client
}

type Result struct {
	Content          string
	Model            string
	ID               string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 0}}
}

func CompletionsURL(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	switch {
	case b == "":
		return ""
	case strings.HasSuffix(b, "/v1/chat/completions"), strings.HasSuffix(b, "/chat/completions"):
		return b
	case strings.HasSuffix(b, "/v1"):
		return b + "/chat/completions"
	default:
		return b + "/v1/chat/completions"
	}
}

func BaseURL(raw string) string {
	b := strings.TrimRight(strings.TrimSpace(raw), "/")
	b = strings.TrimSuffix(b, "/v1/chat/completions")
	b = strings.TrimSuffix(b, "/chat/completions")
	b = strings.TrimSuffix(b, "/v1")
	return strings.TrimRight(b, "/")
}

func (c *Client) Complete(ctx context.Context, base, token, model, system, user string, jsonMode bool) (*Result, error) {
	url := CompletionsURL(base)
	if url == "" {
		return nil, fmt.Errorf("未配置本地 Chat 地址")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("未配置本地 Chat 模型")
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.7,
		"think":       false,
		"max_tokens":  8192,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
		payload["max_tokens"] = 16384
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("模型响应超时，请重试或换本地模型")
		}
		return nil, fmt.Errorf("连接本地 Chat 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("本地 Chat 错误 HTTP %d: %s", resp.StatusCode, clip(raw, 400))
	}
	var sr struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, fmt.Errorf("无法解析 Chat 响应: %s", clip(raw, 200))
	}
	if len(sr.Choices) == 0 {
		return nil, fmt.Errorf("模型没有返回内容")
	}
	content := strings.TrimSpace(sr.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(sr.Choices[0].Message.ReasoningContent)
	}
	if content == "" {
		return nil, fmt.Errorf("模型没有返回内容")
	}
	out := &Result{
		Content:          content,
		Model:            firstNonEmpty(sr.Model, model),
		ID:               sr.ID,
		PromptTokens:     sr.Usage.PromptTokens,
		CompletionTokens: sr.Usage.CompletionTokens,
		TotalTokens:      sr.Usage.TotalTokens,
	}
	return out, nil
}

func (c *Client) Inspect(base, token string) models.SidecarHealth {
	start := time.Now()
	modelsList, err := c.ListModels(base, token)
	lat := time.Since(start).Milliseconds()
	if err != nil {
		return models.SidecarHealth{LatencyMS: lat, Detail: "离线"}
	}
	detail := "在线"
	if pick := PickPlotModel(modelsList, ""); pick != "" {
		detail = pick
		if len(modelsList) > 1 {
			detail += fmt.Sprintf(" 等 %d 个模型", len(modelsList))
		}
	}
	return models.SidecarHealth{Ready: true, LatencyMS: lat, Detail: detail, TextEncoderLabel: "本地 Chat"}
}

func (c *Client) ListModels(base, token string) ([]string, error) {
	root := BaseURL(base)
	if root == "" {
		return nil, fmt.Errorf("未配置地址")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if ids, err := c.getJSON(ctx, token, root+"/v1/models"); err == nil {
		if data, ok := ids["data"].([]any); ok {
			for _, item := range data {
				m, _ := item.(map[string]any)
				add(fmt.Sprint(m["id"]))
			}
		}
	}
	if tags, err := c.getJSON(ctx, token, root+"/api/tags"); err == nil {
		if list, ok := tags["models"].([]any); ok {
			for _, item := range list {
				m, _ := item.(map[string]any)
				add(firstNonEmpty(fmt.Sprint(m["name"]), fmt.Sprint(m["model"])))
			}
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/health", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil, nil
	}
	return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
}

func (c *Client) getJSON(ctx context.Context, token, url string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" && v != "<nil>" {
			return v
		}
	}
	return ""
}
