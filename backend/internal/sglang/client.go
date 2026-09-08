package sglang

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

type VideoRequest struct {
	Model               string      `json:"model"`
	Prompt              string      `json:"prompt"`
	Seconds             float64     `json:"seconds"`
	Task                string      `json:"task"`
	Conditions          []Condition `json:"conditions"`
	Target              Target      `json:"target"`
	Quality             string      `json:"quality,omitempty"`
	NumOutputsPerPrompt int         `json:"num_outputs_per_prompt"`
	NumInferenceSteps   int         `json:"num_inference_steps"`
	FlowShift           float64     `json:"flow_shift"`
	AudioFlowShift      float64     `json:"audio_flow_shift"`
	Seed                int64       `json:"seed"`
}

type FastH3Request struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	Seconds           int     `json:"seconds"`
	Size              string  `json:"size,omitempty"`
	NumFrames         int     `json:"num_frames,omitempty"`
	Seed              int64   `json:"seed"`
	NumInferenceSteps int     `json:"num_inference_steps"`
	GuidanceScale     float64 `json:"guidance_scale"`
}

type Condition struct {
	Type             string   `json:"type"`
	URI              string   `json:"uri"`
	Role             string   `json:"role"`
	FrameIndex       *int     `json:"frame_index,omitempty"`
	StartTimeSeconds *float64 `json:"start_time_seconds,omitempty"`
}

type Target struct {
	ShortEdge       int     `json:"short_edge"`
	AspectRatio     string  `json:"aspect_ratio"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type CreateResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
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

func (c *Client) Create(base string, body VideoRequest) (*CreateResponse, error) {
	return c.CreateJSON(base, body)
}

func (c *Client) CreateJSON(base string, body any) (*CreateResponse, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/v1/videos", bytes.NewReader(raw))
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
		return nil, fmt.Errorf("sglang create %d: %s", resp.StatusCode, clip(b, 800))
	}
	var out CreateResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode create: %w (%s)", err, clip(b, 400))
	}
	if out.ID == "" {
		return nil, fmt.Errorf("sglang 未返回任务 id: %s", clip(b, 400))
	}
	return &out, nil
}

func (c *Client) Status(base, id string) (*StatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/videos/"+id, nil)
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
		return nil, fmt.Errorf("sglang status %d: %s", resp.StatusCode, clip(b, 800))
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
	base = strings.TrimRight(base, "/")
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/videos/"+id+"/cancel", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		del, err := http.NewRequest(http.MethodDelete, base+"/v1/videos/"+id, nil)
		if err != nil {
			return err
		}
		resp, err = client.Do(del)
		if err != nil {
			return err
		}
		b, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 300 {
			return nil
		}
	}
	return fmt.Errorf("sglang cancel %d: %s", resp.StatusCode, clip(b, 400))
}

func (c *Client) Download(base, id, dest string) (int64, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/videos/"+id+"/content", nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("sglang download %d: %s", resp.StatusCode, clip(b, 800))
	}
	return writeFile(dest, resp.Body)
}

func clip(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
