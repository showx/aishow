package minimax

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

type VideoGenRequest struct {
	Model      string        `json:"model"`
	Content    []ContentPart `json:"content"`
	Resolution string        `json:"resolution"`
	Duration   int           `json:"duration"`
	Ratio      string        `json:"ratio,omitempty"`
}

type TaskQuery struct {
	TaskID string `json:"task_id"`
	Task   struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Content struct {
			Prompt string `json:"prompt"`
			URL    string `json:"url"`
		} `json:"content"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"task"`
	BaseResp *struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Client) CreateContextIR(base, token string, body ContextIRRequest) (string, error) {
	return c.createTask(base, token, "/v2/h3_context_ir", body, "context-ir")
}

func (c *Client) CreateVideo(base, token string, body VideoGenRequest) (string, error) {
	return c.createTask(base, token, "/v2/video_generation", body, "video")
}

func (c *Client) createTask(base, token, path string, body any, kind string) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+path, bytes.NewReader(raw))
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
		return "", fmt.Errorf("%s %d: %s", kind, resp.StatusCode, apiError(b))
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
	return "", fmt.Errorf("%s 未返回 task_id: %s", kind, clip(b, 400))
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
		return nil, fmt.Errorf("query %d: %s", resp.StatusCode, apiError(b))
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
			if q.Task.Error != nil && q.Task.Error.Message != "" {
				msg = q.Task.Error.Message
			} else if q.BaseResp != nil && q.BaseResp.StatusMsg != "" {
				msg = q.BaseResp.StatusMsg
			}
			return "", fmt.Errorf("%s", msg)
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("等待 context-ir 超时")
}

func (c *Client) CancelTask(base, token, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(base, "/")+"/v2/video_generation/"+taskID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("cancel %d: %s", resp.StatusCode, apiError(b))
	}
	return nil
}

func (c *Client) DownloadURL(url, dest string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("download %d: %s", resp.StatusCode, clip(b, 400))
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, resp.Body)
}

func (c *Client) Health(base, token string) (bool, int64, string) {
	base = strings.TrimRight(base, "/")
	start := time.Now()
	if token == "" {
		return false, 0, "未配置 Token"
	}
	req, err := http.NewRequest(http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return false, time.Since(start).Milliseconds(), err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, time.Since(start).Milliseconds(), err.Error()
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	lat := time.Since(start).Milliseconds()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, lat, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return false, lat, "Token 无效"
	}
	return resp.StatusCode < 500, lat, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func EncodeDataURI(path, mime string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if mime == "" {
		mime = mimeFromName(path)
	}
	if !strings.Contains(mime, "/") {
		mime = "image/" + strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	}
	return "data:" + strings.ToLower(mime) + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func mimeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mp3"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

func apiError(b []byte) string {
	var parsed struct {
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		BaseResp *struct {
			StatusMsg string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if json.Unmarshal(b, &parsed) == nil {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return parsed.Error.Message
		}
		if parsed.BaseResp != nil && parsed.BaseResp.StatusMsg != "" {
			return parsed.BaseResp.StatusMsg
		}
	}
	return clip(b, 400)
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
