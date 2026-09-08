package models

import (
	"strings"
	"time"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"

	ModeT2VA   = "t2va"
	ModeI2VA   = "i2va"
	ModeL2VA   = "l2va"
	ModeFL2VA  = "fl2va"
	ModeRef2VA = "ref2va"
	ModeT2I    = "t2i"
	ModeI2I    = "i2i"

	EngineH3         = "h3"
	EngineFastH3     = "fasth3"
	EngineH3Max      = "h3-max"
	EngineLLadaImage = "llada-image"
)

type Job struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	Title           string     `gorm:"size:200" json:"title"`
	Mode            string     `gorm:"size:32;index" json:"mode"`
	Engine          string     `gorm:"size:32;index" json:"engine"`
	Status          string     `gorm:"size:32;index" json:"status"`
	Priority        int        `gorm:"index" json:"priority"`
	Prompt          string     `gorm:"type:text" json:"prompt"`
	EnhancedPrompt  string     `gorm:"type:text" json:"enhanced_prompt"`
	Duration        float64    `json:"duration"`
	AspectRatio     string     `gorm:"size:32" json:"aspect_ratio"`
	ShortEdge       int        `json:"short_edge"`
	Seed            int64      `json:"seed"`
	Steps           int        `json:"steps"`
	FlowShift       float64    `json:"flow_shift"`
	AudioFlowShift  float64    `json:"audio_flow_shift"`
	Quality         string     `gorm:"size:32" json:"quality"`
	EnhancePrompt   bool       `json:"enhance_prompt"`
	Outputs         int        `json:"outputs"`
	Progress        int        `json:"progress"`
	Stage           string     `gorm:"size:80" json:"stage"`
	ErrorMessage    string     `gorm:"type:text" json:"error_message"`
	RemoteID        string     `gorm:"size:128" json:"remote_id"`
	OutputPath      string     `gorm:"size:500" json:"-"`
	OutputSize      int64      `json:"output_size"`
	HasVideo        bool       `json:"has_video"`
	HasImage        bool       `json:"has_image"`
	QueuePosition   int        `gorm:"-" json:"queue_position"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	Assets          []JobAsset `gorm:"foreignKey:JobID" json:"assets"`
	Events          []JobEvent `gorm:"foreignKey:JobID" json:"events,omitempty"`
}

type JobAsset struct {
	ID           string   `gorm:"primaryKey;size:36" json:"id"`
	JobID        string   `gorm:"size:36;index" json:"job_id"`
	UploadID     string   `gorm:"size:36;index" json:"upload_id"`
	Role         string   `gorm:"size:32" json:"role"`
	Type         string   `gorm:"size:32" json:"type"`
	FrameIndex   *int     `json:"frame_index"`
	StartSeconds *float64 `json:"start_seconds"`
	URI          string   `gorm:"size:800" json:"uri"`
	Filename     string   `gorm:"size:260" json:"filename"`
}

type Upload struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Filename  string    `gorm:"size:260" json:"filename"`
	Mime      string    `gorm:"size:120" json:"mime"`
	Size      int64     `json:"size"`
	Kind      string    `gorm:"size:32" json:"kind"`
	Path      string    `gorm:"size:500" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type Setting struct {
	Key   string `gorm:"primaryKey;size:80" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

type JobEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JobID     string    `gorm:"size:36;index" json:"job_id"`
	Level     string    `gorm:"size:16" json:"level"`
	Message   string    `gorm:"type:text" json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateJobRequest struct {
	Title          string            `json:"title"`
	Mode           string            `json:"mode" binding:"required"`
	Engine         string            `json:"engine"`
	Prompt         string            `json:"prompt" binding:"required"`
	Duration       float64           `json:"duration"`
	AspectRatio    string            `json:"aspect_ratio"`
	ShortEdge      int               `json:"short_edge"`
	Seed           *int64            `json:"seed"`
	Steps          int               `json:"steps"`
	FlowShift      float64           `json:"flow_shift"`
	AudioFlowShift float64           `json:"audio_flow_shift"`
	Quality        string            `json:"quality"`
	EnhancePrompt  bool              `json:"enhance_prompt"`
	Outputs        int               `json:"outputs"`
	Priority       int               `json:"priority"`
	Conditions     []AssetCondition  `json:"conditions"`
}

type AssetCondition struct {
	UploadID     string   `json:"upload_id"`
	Role         string   `json:"role"`
	Type         string   `json:"type"`
	FrameIndex   *int     `json:"frame_index"`
	StartSeconds *float64 `json:"start_seconds"`
}

type SettingsPayload struct {
	InferenceMode     string `json:"inference_mode"`
	SGLANGFL2VAURL    string `json:"sglang_fl2va_url"`
	SGLANGRef2VAURL   string `json:"sglang_ref2va_url"`
	MediaFilePrefix   string `json:"media_file_prefix"`
	URIMode           string `json:"uri_mode"`
	WorkerConcurrency int    `json:"worker_concurrency"`
	PublicBaseURL     string `json:"public_base_url"`
	MiniMaxAPIBase    string `json:"minimax_api_base"`
	MiniMaxAPIToken   string `json:"minimax_api_token"`
	HasMiniMaxToken   bool   `json:"has_minimax_token"`
	FastH3URL         string `json:"fasth3_url"`
	LLaDAImageURL     string `json:"llada_image_url"`
}

type SystemStatus struct {
	App            string           `json:"app"`
	Version        string           `json:"version"`
	InferenceMode  string           `json:"inference_mode"`
	QueueDepth     int64            `json:"queue_depth"`
	Running        int64            `json:"running"`
	SucceededToday int64            `json:"succeeded_today"`
	FailedToday    int64            `json:"failed_today"`
	TotalJobs      int64            `json:"total_jobs"`
	WorkerSlots    int              `json:"worker_slots"`
	Endpoints      []EndpointHealth `json:"endpoints"`
	Hardware       Hardware         `json:"hardware"`
	Time           time.Time        `json:"time"`
}

type Hardware struct {
	CPUPercent     float64   `json:"cpu_percent"`
	CPUCores       int       `json:"cpu_cores"`
	RAMUsedGB      float64   `json:"ram_used_gb"`
	RAMTotalGB     float64   `json:"ram_total_gb"`
	RAMPercent     float64   `json:"ram_percent"`
	GPUName        string    `json:"gpu_name"`
	GPUPercent     int       `json:"gpu_percent"`
	GPUMemUsedMB   int       `json:"gpu_mem_used_mb"`
	GPUMemTotalMB  int       `json:"gpu_mem_total_mb"`
	GPUTemp        int       `json:"gpu_temp"`
	GPUs           []GPUStat `json:"gpus"`
}

type GPUStat struct {
	Name       string `json:"name"`
	Util       int    `json:"util"`
	MemUsedMB  int    `json:"mem_used_mb"`
	MemTotalMB int    `json:"mem_total_mb"`
	Temp       int    `json:"temp"`
}

type EndpointHealth struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Healthy   bool   `json:"healthy"`
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
}

func IsFastH3(engine string) bool {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case EngineFastH3, EngineH3Max, "fast-h3", "fast_h3", "h3max", "h3_max":
		return true
	default:
		return false
	}
}

func IsImageEngine(engine string) bool {
	switch engine {
	case EngineLLadaImage, "llada", "llada_image":
		return true
	default:
		return false
	}
}

func IsImageMode(mode string) bool {
	return mode == ModeT2I || mode == ModeI2I
}

func IsVideoMode(mode string) bool {
	switch mode {
	case ModeT2VA, ModeI2VA, ModeL2VA, ModeFL2VA, ModeRef2VA:
		return true
	default:
		return false
	}
}
