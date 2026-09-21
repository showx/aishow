package models

import "time"

type DramaShotBeat struct {
	Index       int    `json:"index"`
	TimeRange   string `json:"time_range"`
	Action      string `json:"action"`
	ImagePrompt string `json:"image_prompt,omitempty"`
}

type DramaImageRef struct {
	UploadID string `json:"upload_id"`
	Name     string `json:"name,omitempty"`
	Kind     string `json:"kind,omitempty"` // character | scene
}

type DramaImageRefView struct {
	DramaImageRef
	URL string `json:"url,omitempty"`
}

type DramaShot struct {
	Index             int             `json:"index"`
	Title             string          `json:"title"`
	Scene             string          `json:"scene"`
	Dialogue          string          `json:"dialogue,omitempty"`
	ImagePrompt       string          `json:"image_prompt"`
	VideoPrompt       string          `json:"video_prompt"`
	Duration          float64         `json:"duration"`
	ImageJobID        string          `json:"image_job_id,omitempty"`
	ExtraImageJobIDs  []string        `json:"extra_image_job_ids,omitempty"`
	VideoJobID        string          `json:"video_job_id,omitempty"`
	ImageUploadID     string          `json:"image_upload_id,omitempty"`
	VideoUploadID     string          `json:"video_upload_id,omitempty"`
	LastFrameUploadID string          `json:"last_frame_upload_id,omitempty"`
	SkipPrevImages    bool            `json:"skip_prev_images,omitempty"`
	QueueImage        bool            `json:"queue_image,omitempty"`
	QueueVideo        bool            `json:"queue_video,omitempty"`
	ContinueFromPrev  bool            `json:"continue_from_prev,omitempty"`
	ImageRefs         []DramaImageRef `json:"image_refs,omitempty"`
	Beats             []DramaShotBeat `json:"beats,omitempty"`
}

type DramaShotView struct {
	DramaShot
	ImageJob       *Job                `json:"image_job,omitempty"`
	ExtraImageJobs []Job               `json:"extra_image_jobs,omitempty"`
	VideoJob       *Job                `json:"video_job,omitempty"`
	ImageURL       string              `json:"image_url,omitempty"`
	ImageURLs      []string            `json:"image_urls,omitempty"`
	VideoURL       string              `json:"video_url,omitempty"`
	LastFrameURL   string              `json:"last_frame_url,omitempty"`
	ImageRefViews  []DramaImageRefView `json:"image_ref_views,omitempty"`
}

type DramaProject struct {
	ID         string `gorm:"primaryKey;size:36" json:"id"`
	UserID     string `gorm:"size:36;index" json:"user_id"`
	Title      string `gorm:"size:200" json:"title"`
	Idea       string `gorm:"type:text" json:"idea"`
	Style      string `gorm:"type:text" json:"style"`
	StyleNotes string `gorm:"type:text" json:"style_notes"`
	TargetSec  int    `json:"target_sec"`
	Step       string `gorm:"size:32" json:"step"`
	Status     string `gorm:"size:32;index" json:"status"`

	ScriptText string `gorm:"type:text" json:"script_text"`
	ShotsJSON  string `gorm:"type:text" json:"shots_json"`

	WriteModel       string `gorm:"size:120" json:"write_model"`
	WriteResult      string `gorm:"type:text" json:"write_result"`
	StoryboardModel  string `gorm:"size:120" json:"storyboard_model"`
	StoryboardResult string `gorm:"type:text" json:"storyboard_result"`

	ImageEngine    string `gorm:"size:32" json:"image_engine"`
	ImageAspect    string `gorm:"size:16" json:"image_aspect"`
	ImageShortEdge int    `json:"image_short_edge"`
	ImageQuality   string `gorm:"size:32" json:"image_quality"`

	VideoEngine    string  `gorm:"size:32" json:"video_engine"`
	ContinueEngine string  `gorm:"size:32" json:"continue_engine"`
	VideoAspect    string  `gorm:"size:16" json:"video_aspect"`
	VideoShortEdge int     `json:"video_short_edge"`
	VideoDuration  float64 `json:"video_duration"`

	ImageRefsJSON string              `gorm:"type:text" json:"-"`
	ImageRefs     []DramaImageRefView `json:"image_refs,omitempty" gorm:"-"`
	BurnSubtitles bool                `json:"burn_subtitles"`
	MixTTS        bool                `json:"mix_tts"`

	CompileJobID       string `gorm:"size:36" json:"compile_job_id"`
	CompileStatus      string `gorm:"size:32" json:"compile_status"`
	CompileResult      string `gorm:"type:text" json:"compile_result"`
	CompileIndexesJSON string `gorm:"type:text" json:"compile_indexes_json"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Shots          []DramaShotView `json:"shots,omitempty" gorm:"-"`
	CompileIndexes []int           `json:"compile_indexes,omitempty" gorm:"-"`
	CompileURL     string          `json:"compile_url,omitempty" gorm:"-"`
}

func (DramaProject) TableName() string { return "drama_projects" }
