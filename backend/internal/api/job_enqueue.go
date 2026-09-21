package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aishow/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) createJob(c *gin.Context) {
	var in models.CreateJobRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	in.DramaID = ""
	in.DramaShotIndex = 0
	in.DramaKind = ""
	job, err := s.enqueueJob(currentUserID(c), in)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(200, job)
}

func (s *Server) enqueueJob(userID string, in models.CreateJobRequest) (*models.Job, error) {
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if !validMode(mode) {
		return nil, badRequest("不支持的生成模式")
	}
	engine := normalizeEngine(in.Engine)
	if engine == "" {
		return nil, badRequest("不支持的生成引擎")
	}
	if models.IsFastH3(engine) && mode != models.ModeT2VA {
		return nil, badRequest("本地 FastH3 Preview 只支持文生影像")
	}
	if models.IsH3Ref2VAInt8(engine) && mode != models.ModeRef2VA {
		return nil, badRequest("H3 Ref2VA INT8 只支持参考生成")
	}
	if models.IsH3PinkCherryInt8(engine) && mode == models.ModeRef2VA {
		return nil, badRequest("PinkCherry INT8 只承接文生和首尾帧；参考生成请改选「H3 Ref2VA INT8」或「H3 Timeline Director」")
	}
	if models.IsH3Director(engine) && mode != models.ModeT2VA && mode != models.ModeRef2VA {
		return nil, badRequest("H3 Timeline Director 只支持文生和参考生成")
	}
	if (engine == models.EngineH3 || models.IsH3Turbo(engine)) && mode == models.ModeRef2VA {
		return nil, badRequest("参考生成请改选「H3 Ref2VA INT8」或「H3 Timeline Director」；H3-Base / Turbo LoRA 只承接文生和首尾帧")
	}
	if models.IsImageEngine(engine) && !models.IsImageMode(mode) {
		return nil, badRequest("LLaDA-Image 仅支持文生图与指令编辑")
	}
	if !models.IsImageEngine(engine) && models.IsImageMode(mode) {
		return nil, badRequest("文生图 / 指令编辑请选择 LLaDA-Image")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return nil, badRequest("请填写提示词")
	}
	if models.IsImageEngine(engine) {
		in.Duration = 0
		in.EnhancePrompt = false
		if in.Quality == "" {
			in.Quality = "turbo"
		}
		if in.ShortEdge <= 0 {
			in.ShortEdge = 1024
		}
		if in.ShortEdge < 512 {
			in.ShortEdge = 512
		}
		if in.ShortEdge > 1536 {
			in.ShortEdge = 1536
		}
		if in.AspectRatio == "" || in.AspectRatio == "auto" {
			in.AspectRatio = "1:1"
		}
		if in.Steps == 0 {
			if strings.EqualFold(in.Quality, "base") {
				in.Steps = 50
			} else {
				in.Steps = 4
			}
		}
		if in.FlowShift == 0 {
			if strings.EqualFold(in.Quality, "base") {
				in.FlowShift = 5
			} else {
				in.FlowShift = 1
			}
		}
	} else {
		if in.Duration <= 0 {
			in.Duration = 5
		} else if in.Duration < 2 {
			in.Duration = 2
		}
		if models.IsH3Director(engine) {
			if in.Duration > 30 {
				in.Duration = 30
			}
		} else if in.Duration > 15 {
			in.Duration = 15
		}
		if in.ShortEdge <= 0 {
			in.ShortEdge = 480
		}
		if models.IsFastH3(engine) {
			if in.ShortEdge >= 640 {
				in.ShortEdge = 768
			} else {
				in.ShortEdge = 480
			}
			in.Steps = 5
		}
		if models.IsH3Turbo(engine) {
			if in.ShortEdge >= 640 {
				in.ShortEdge = 768
			} else {
				in.ShortEdge = 480
			}
			if in.Steps == 0 || in.Steps == 50 {
				in.Steps = 4
			}
			if in.Steps < 4 {
				in.Steps = 4
			}
			if in.Steps > 8 {
				in.Steps = 8
			}
			if in.FlowShift == 0 || in.FlowShift == 12 {
				in.FlowShift = 6
			}
			if in.Quality == "" || in.Quality == "lossless" {
				in.Quality = "turbo"
			}
		}
		if models.IsH3Ref2VAInt8(engine) && (in.Steps == 0 || in.Steps == 50) {
			in.Steps = 20
		}
		if models.IsH3Director(engine) {
			if in.ShortEdge >= 640 {
				in.ShortEdge = 768
			} else {
				in.ShortEdge = 480
			}
			if in.Steps == 0 || in.Steps == 50 {
				in.Steps = 8
			}
			if in.Steps < 4 {
				in.Steps = 4
			}
			if in.Steps > 50 {
				in.Steps = 50
			}
		}
		if models.IsH3PinkCherryInt8(engine) {
			if in.ShortEdge >= 640 {
				in.ShortEdge = 768
			} else {
				in.ShortEdge = 480
			}
			if in.Steps == 0 || in.Steps == 50 {
				in.Steps = 20
			}
			if in.Steps < 8 {
				in.Steps = 8
			}
			if in.Steps > 50 {
				in.Steps = 50
			}
		}
		if in.ShortEdge < 256 {
			in.ShortEdge = 256
		}
		if in.ShortEdge > 1080 {
			in.ShortEdge = 1080
		}
		if in.AspectRatio == "" {
			in.AspectRatio = "16:9"
		}
		if (models.IsFastH3(engine) || models.IsH3Turbo(engine) || models.IsH3PinkCherryInt8(engine)) && in.AspectRatio == "auto" {
			in.AspectRatio = "16:9"
		}
		if in.Steps == 0 {
			in.Steps = 50
		}
		if in.FlowShift == 0 {
			in.FlowShift = 12
		}
	}
	if in.AudioFlowShift == 0 {
		in.AudioFlowShift = 3
	}
	if in.Quality == "" {
		in.Quality = "lossless"
	}
	if in.Outputs < 1 {
		in.Outputs = 1
	}
	seed := time.Now().Unix() % 100000
	if in.Seed != nil {
		seed = *in.Seed
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = clipRunes(in.Prompt, 18)
	}
	textID, textLabel := models.DescribeTextEncoder(engine, "")
	job := models.Job{
		ID:               uuid.NewString(),
		UserID:           userID,
		Title:            title,
		Mode:             mode,
		Engine:           engine,
		Status:           models.StatusQueued,
		Priority:         in.Priority,
		Prompt:           in.Prompt,
		TextEncoder:      textID,
		TextEncoderLabel: textLabel,
		PromptRewriter:   models.PromptRewriterFor(engine, in.EnhancePrompt),
		Duration:         in.Duration,
		AspectRatio:      in.AspectRatio,
		ShortEdge:        in.ShortEdge,
		Seed:             seed,
		Steps:            in.Steps,
		FlowShift:        in.FlowShift,
		AudioFlowShift:   in.AudioFlowShift,
		Quality:          in.Quality,
		EnhancePrompt:    in.EnhancePrompt,
		Outputs:          in.Outputs,
		Stage:            "排队中",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		DramaID:          strings.TrimSpace(in.DramaID),
		DramaShotIndex:   in.DramaShotIndex,
		DramaKind:        strings.TrimSpace(in.DramaKind),
	}
	assets, err := s.buildAssets(job.ID, userID, mode, in.Conditions)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	job.Assets = assets
	if err := s.queue.Enqueue(&job); err != nil {
		return nil, err
	}
	s.queue.Log(job.ID, "info", "任务已进入队列")
	job.QueuePosition = s.queue.Position(job)
	return &job, nil
}

func (s *Server) registerJobOutputUpload(userID, jobID, kind string) (*models.Upload, error) {
	var job models.Job
	if err := s.db.First(&job, "id = ? AND user_id = ?", jobID, userID).Error; err != nil {
		return nil, fmt.Errorf("任务不存在")
	}
	src := strings.TrimSpace(job.OutputPath)
	if src == "" {
		if kind == "image" {
			src = s.store.ImageOutputPath(jobID)
		} else {
			src = s.store.OutputPath(jobID)
		}
	}
	if _, err := os.Stat(src); err != nil {
		return nil, nil
	}
	id := uuid.NewString()
	filename := jobID + filepath.Ext(src)
	if filename == jobID {
		if kind == "image" {
			filename = jobID + ".png"
		} else {
			filename = jobID + ".mp4"
		}
	}
	dest, n, err := func() (string, int64, error) {
		f, err := os.Open(src)
		if err != nil {
			return "", 0, err
		}
		defer f.Close()
		return s.store.SaveUpload(id, filename, f)
	}()
	if err != nil {
		return nil, err
	}
	mime := "application/octet-stream"
	if kind == "image" {
		mime = "image/png"
	} else if kind == "video" {
		mime = "video/mp4"
	}
	up := models.Upload{
		ID:        id,
		UserID:    userID,
		Filename:  filename,
		Mime:      mime,
		Size:      n,
		Kind:      kind,
		Path:      dest,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&up).Error; err != nil {
		return nil, err
	}
	return &up, nil
}

func (s *Server) registerLocalUpload(userID, src, filename, kind, mime string) (*models.Upload, error) {
	if _, err := os.Stat(src); err != nil {
		return nil, nil
	}
	id := uuid.NewString()
	if filename == "" {
		filename = filepath.Base(src)
	}
	dest, n, err := func() (string, int64, error) {
		f, err := os.Open(src)
		if err != nil {
			return "", 0, err
		}
		defer f.Close()
		return s.store.SaveUpload(id, filename, f)
	}()
	if err != nil {
		return nil, err
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	up := models.Upload{
		ID:        id,
		UserID:    userID,
		Filename:  filename,
		Mime:      mime,
		Size:      n,
		Kind:      kind,
		Path:      dest,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&up).Error; err != nil {
		return nil, err
	}
	return &up, nil
}
