package llada

import (
	"bytes"
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

type ImageRequest struct {
	Prompt            string      `json:"prompt"`
	NegativePrompt    string      `json:"negative_prompt,omitempty"`
	Task              string      `json:"task"`
	Conditions        []Condition `json:"conditions,omitempty"`
	Quality           string      `json:"quality,omitempty"`
	NumInferenceSteps int         `json:"num_inference_steps"`
	GuidanceScale     float64     `json:"guidance_scale"`
	Seed              int64       `json:"seed"`
	Target            Target      `json:"target"`
}

type Condition struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
	Role string `json:"role"`
}

type Target struct {
	ShortEdge   int    `json:"short_edge"`
	AspectRatio string `json:"aspect_ratio"`
}

type CreateResponse struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	TextEncoder      string `json:"text_encoder"`
	TextEncoderLabel string `json:"text_encoder_label"`
}

type StatusResponse struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Error    *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Health(base string) (bool, int64, string) {
	base = strings.TrimRight(base, "/")
	start := time.Now()
	urls := []string{base + "/health", base + "/v1/models", base + "/"}
	var last string
	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			last = err.Error()
			continue
		}
		resp, err := c.http.Do(req)
		if err != nil {
			last = err.Error()
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lat := time.Since(start).Milliseconds()
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return true, lat, fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		last = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, time.Since(start).Milliseconds(), last
}

func (c *Client) Create(base string, body ImageRequest) (*CreateResponse, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/v1/images", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llada create %d: %s", resp.StatusCode, clip(b, 800))
	}
	var out CreateResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode create: %w (%s)", err, clip(b, 400))
	}
	if out.ID == "" {
		return nil, fmt.Errorf("LLaDA-Image 未返回任务 id: %s", clip(b, 400))
	}
	return &out, nil
}

func (c *Client) Status(base, id string) (*StatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/images/"+id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llada status %d: %s", resp.StatusCode, clip(b, 800))
	}
	var out StatusResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Cancel(base, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/v1/images/"+id+"/cancel", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("llada cancel %d: %s", resp.StatusCode, clip(b, 400))
	}
	return nil
}

func (c *Client) Download(base, id, dest string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/images/"+id+"/content", nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("llada download %d: %s", resp.StatusCode, clip(b, 800))
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

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
