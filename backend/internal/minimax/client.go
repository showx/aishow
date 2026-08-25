package minimax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	http *http.Client
}

type ContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *URLPart       `json:"image_url,omitempty"`
	VideoURL *URLPart       `json:"video_url,omitempty"`
	AudioURL *URLPart       `json:"audio_url,omitempty"`
	Role     string         `json:"role,omitempty"`
}

type URLPart struct {
	URL string `json:"url"`
}

type ContextIRRequest struct {
	Model    string        `json:"model"`
	Content  []ContentPart `json:"content"`
	Duration float64       `json:"duration"`
	Ratio    string        `json:"ratio"`
}

type TaskQuery struct {
	TaskID string `json:"task_id"`
	Task   struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Content struct {
			Prompt string `json:"prompt"`
		} `json:"content"`
	} `json:"task"`
	BaseResp *struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) CreateContextIR(base, token string, body ContextIRRequest) (string, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/v2/h3_context_ir", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("context-ir %d: %s", resp.StatusCode, string(b))
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	if id, ok := parsed["task_id"].(string); ok && id != "" {
		return id, nil
	}
	if task, ok := parsed["task"].(map[string]any); ok {
		if id, ok := task["id"].(string); ok && id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("context-ir 未返回 task_id: %s", string(b))
}

func (c *Client) QueryTask(base, token, taskID string) (*TaskQuery, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v2/query/video_generation/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("query %d: %s", resp.StatusCode, string(b))
	}
	var out TaskQuery
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) WaitPrompt(base, token, taskID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		q, err := c.QueryTask(base, token, taskID)
		if err != nil {
			return "", err
		}
		status := strings.ToLower(q.Task.Status)
		switch status {
		case "succeeded", "success", "completed":
			if q.Task.Content.Prompt == "" {
				return "", fmt.Errorf("context-ir 成功但未返回 prompt")
			}
			return q.Task.Content.Prompt, nil
		case "failed", "error":
			msg := "context-ir 失败"
			if q.BaseResp != nil && q.BaseResp.StatusMsg != "" {
				msg = q.BaseResp.StatusMsg
			}
			return "", fmt.Errorf("%s", msg)
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("等待 context-ir 超时")
}
