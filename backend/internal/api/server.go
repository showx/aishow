package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"aishow/internal/config"
	"aishow/internal/hub"
	"aishow/internal/llada"
	"aishow/internal/metrics"
	"aishow/internal/models"
	"aishow/internal/queue"
	"aishow/internal/settings"
	"aishow/internal/sglang"
	"aishow/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Server struct {
	cfg   config.Config
	db    *gorm.DB
	queue *queue.Service
	store *storage.Store
	hub   *hub.Hub
	sg    *sglang.Client
	ll    *llada.Client
	hw    *metrics.Collector
}

func New(cfg config.Config, db *gorm.DB, q *queue.Service, store *storage.Store, h *hub.Hub, hw *metrics.Collector) *Server {
	return &Server{cfg: cfg, db: db, queue: q, store: store, hub: h, sg: sglang.New(), ll: llada.New(), hw: hw}
}

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	{
		api.GET("/health", s.health)
		api.GET("/system", s.system)
		api.GET("/metrics", s.metrics)
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.putSettings)
		api.GET("/events", s.events)

		api.POST("/uploads", s.createUpload)
		api.GET("/uploads/:id", s.getUpload)
		api.GET("/uploads/:id/raw", s.rawUpload)

		api.POST("/jobs", s.createJob)
		api.GET("/jobs", s.listJobs)
		api.GET("/jobs/:id", s.getJob)
		api.POST("/jobs/:id/cancel", s.cancelJob)
		api.POST("/jobs/:id/retry", s.retryJob)
		api.POST("/jobs/:id/bump", s.bumpJob)
		api.DELETE("/jobs/:id", s.deleteJob)
		api.GET("/jobs/:id/video", s.jobVideo)
		api.GET("/jobs/:id/image", s.jobImage)
		api.GET("/jobs/:id/events", s.jobEvents)

		api.GET("/media/:jobId/:name", s.mediaFile)
	}

	dist := filepath.Clean(filepath.Join("..", "frontend", "dist"))
	if info, err := os.Stat(dist); err == nil && info.IsDir() {
		r.Static("/assets", filepath.Join(dist, "assets"))
		r.StaticFile("/favicon.svg", filepath.Join(dist, "favicon.svg"))
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.File(filepath.Join(dist, "index.html"))
		})
	}
	return r
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "app": "aishow"})
}

func (s *Server) system(c *gin.Context) {
	snap := settings.Snapshot(s.db, s.cfg)
	today := time.Now().Truncate(24 * time.Hour)
	st := models.SystemStatus{
		App:           "Aishow",
		Version:       "0.1.0",
		InferenceMode: snap.InferenceMode,
		WorkerSlots:   snap.WorkerConcurrency,
		Time:          time.Now(),
	}
	s.db.Model(&models.Job{}).Where("status = ?", models.StatusQueued).Count(&st.QueueDepth)
	s.db.Model(&models.Job{}).Where("status = ?", models.StatusRunning).Count(&st.Running)
	s.db.Model(&models.Job{}).Where("status = ? AND finished_at >= ?", models.StatusSucceeded, today).Count(&st.SucceededToday)
	s.db.Model(&models.Job{}).Where("status = ? AND finished_at >= ?", models.StatusFailed, today).Count(&st.FailedToday)
	s.db.Model(&models.Job{}).Count(&st.TotalJobs)

	flOK, flLat, flDet := s.sg.Health(snap.SGLANGFL2VAURL)
	rfOK, rfLat, rfDet := s.sg.Health(snap.SGLANGRef2VAURL)
	fhOK, fhLat, fhDet := s.sg.Health(snap.FastH3URL)
	llOK, llLat, llDet := s.ll.Health(snap.LLaDAImageURL)
	st.Endpoints = []models.EndpointHealth{
		{Name: "H3-Base FL2VA", URL: snap.SGLANGFL2VAURL, Healthy: flOK, LatencyMS: flLat, Detail: flDet},
		{Name: "H3-Base Ref2VA", URL: snap.SGLANGRef2VAURL, Healthy: rfOK, LatencyMS: rfLat, Detail: rfDet},
		{Name: "FastH3 · FastVideo", URL: snap.FastH3URL, Healthy: fhOK, LatencyMS: fhLat, Detail: fhDet},
		{Name: "LLaDA-Image", URL: snap.LLaDAImageURL, Healthy: llOK, LatencyMS: llLat, Detail: llDet},
	}
	st.Hardware = s.hw.Snapshot()
	c.JSON(http.StatusOK, st)
}

func (s *Server) metrics(c *gin.Context) {
	c.JSON(http.StatusOK, s.hw.Snapshot())
}

func (s *Server) getSettings(c *gin.Context) {
	c.JSON(http.StatusOK, settings.Snapshot(s.db, s.cfg))
}

func (s *Server) putSettings(c *gin.Context) {
	var in models.SettingsPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.WorkerConcurrency < 1 {
		in.WorkerConcurrency = 1
	}
	if err := settings.Apply(s.db, in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings.Snapshot(s.db, s.cfg))
}

func (s *Server) events(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)
	c.SSEvent("hello", gin.H{"ok": true})
	c.Writer.Flush()
	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			c.Writer.Write([]byte("event: message\n"))
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(payload)
			c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		}
	}
}

func (s *Server) createUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()
	id := uuid.NewString()
	path, n, err := s.store.SaveUpload(id, file.Filename, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	mime := file.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	up := models.Upload{
		ID:        id,
		Filename:  file.Filename,
		Mime:      mime,
		Size:      n,
		Kind:      kindOf(mime, file.Filename),
		Path:      path,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&up).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, up)
}

func (s *Server) getUpload(c *gin.Context) {
	var up models.Upload
	if err := s.db.First(&up, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, up)
}

func (s *Server) rawUpload(c *gin.Context) {
	var up models.Upload
	if err := s.db.First(&up, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.File(up.Path)
}

func (s *Server) mediaFile(c *gin.Context) {
	path := filepath.Join(s.store.JobMediaDir(c.Param("jobId")), filepath.Base(c.Param("name")))
	c.File(path)
}

func (s *Server) createJob(c *gin.Context) {
	var in models.CreateJobRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if !validMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的生成模式"})
		return
	}
	engine := normalizeEngine(in.Engine)
	if engine == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的生成引擎"})
		return
	}
	if models.IsFastH3(engine) && mode != models.ModeT2VA {
		c.JSON(http.StatusBadRequest, gin.H{"error": "本地 FastH3 Preview 只支持文生影像"})
		return
	}
	if models.IsImageEngine(engine) && !models.IsImageMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LLaDA-Image 仅支持文生图与指令编辑"})
		return
	}
	if !models.IsImageEngine(engine) && models.IsImageMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文生图 / 指令编辑请选择 LLaDA-Image"})
		return
	}
	if strings.TrimSpace(in.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写提示词"})
		return
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
		if in.Duration > 15 {
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
		if in.ShortEdge < 256 {
			in.ShortEdge = 256
		}
		if in.ShortEdge > 1080 {
			in.ShortEdge = 1080
		}
		if in.AspectRatio == "" {
			in.AspectRatio = "16:9"
		}
		if models.IsFastH3(engine) && in.AspectRatio == "auto" {
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
	job := models.Job{
		ID:             uuid.NewString(),
		Title:          title,
		Mode:           mode,
		Engine:         engine,
		Status:         models.StatusQueued,
		Priority:       in.Priority,
		Prompt:         in.Prompt,
		Duration:       in.Duration,
		AspectRatio:    in.AspectRatio,
		ShortEdge:      in.ShortEdge,
		Seed:           seed,
		Steps:          in.Steps,
		FlowShift:      in.FlowShift,
		AudioFlowShift: in.AudioFlowShift,
		Quality:        in.Quality,
		EnhancePrompt:  in.EnhancePrompt,
		Outputs:        in.Outputs,
		Stage:          "排队中",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	assets, err := s.buildAssets(job.ID, mode, in.Conditions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job.Assets = assets
	if err := s.queue.Enqueue(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.queue.Log(job.ID, "info", "任务已进入队列")
	job.QueuePosition = s.queue.Position(job)
	c.JSON(http.StatusOK, job)
}

func (s *Server) buildAssets(jobID, mode string, conds []models.AssetCondition) ([]models.JobAsset, error) {
	var assets []models.JobAsset
	for _, cnd := range conds {
		if cnd.UploadID == "" {
			continue
		}
		var up models.Upload
		if err := s.db.First(&up, "id = ?", cnd.UploadID).Error; err != nil {
			return nil, fmt.Errorf("素材不存在: %s", cnd.UploadID)
		}
		typ := cnd.Type
		if typ == "" {
			typ = up.Kind
		}
		role := cnd.Role
		if role == "" {
			role = "reference"
		}
		assets = append(assets, models.JobAsset{
			ID:           uuid.NewString(),
			JobID:        jobID,
			UploadID:     up.ID,
			Role:         role,
			Type:         typ,
			FrameIndex:   cnd.FrameIndex,
			StartSeconds: cnd.StartSeconds,
			Filename:     up.Filename,
		})
	}
	switch mode {
	case models.ModeI2VA:
		if countRole(assets, "keyframe") == 0 {
			return nil, fmt.Errorf("首帧模式需要一张关键帧")
		}
	case models.ModeL2VA:
		if countRole(assets, "keyframe") == 0 {
			return nil, fmt.Errorf("尾帧模式需要一张关键帧")
		}
	case models.ModeFL2VA:
		if countRole(assets, "keyframe") < 2 {
			return nil, fmt.Errorf("首尾帧模式需要两张关键帧")
		}
	case models.ModeI2I:
		if countRole(assets, "reference") == 0 {
			return nil, fmt.Errorf("指令编辑需要一张参考图")
		}
	}
	return assets, nil
}

func (s *Server) listJobs(c *gin.Context) {
	status := c.Query("status")
	mode := c.Query("mode")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := s.db.Preload("Assets").Order("created_at desc").Limit(limit)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if mode != "" {
		q = q.Where("mode = ?", mode)
	}
	var jobs []models.Job
	if err := q.Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range jobs {
		jobs[i].QueuePosition = s.queue.Position(jobs[i])
	}
	c.JSON(http.StatusOK, jobs)
}

func (s *Server) getJob(c *gin.Context) {
	var job models.Job
	if err := s.db.Preload("Assets").Preload("Events").First(&job, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	job.QueuePosition = s.queue.Position(job)
	c.JSON(http.StatusOK, job)
}

func (s *Server) cancelJob(c *gin.Context) {
	id := c.Param("id")
	var job models.Job
	if err := s.db.First(&job, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if job.RemoteID != "" {
		snap := settings.Snapshot(s.db, s.cfg)
		if models.IsImageEngine(job.Engine) {
			if err := s.ll.Cancel(snap.LLaDAImageURL, job.RemoteID); err != nil {
				s.queue.Log(id, "warn", "通知 LLaDA-Image 中止失败: "+err.Error())
			} else {
				s.queue.Log(id, "info", "已通知 LLaDA-Image 中止")
			}
		} else if models.IsFastH3(job.Engine) {
			if err := s.sg.Cancel(snap.FastH3URL, job.RemoteID); err != nil {
				s.queue.Log(id, "warn", "通知 FastH3 中止失败: "+err.Error())
			} else {
				s.queue.Log(id, "info", "已通知 FastH3 中止")
			}
		} else {
			endpoint := snap.SGLANGFL2VAURL
			if job.Mode == models.ModeRef2VA {
				endpoint = snap.SGLANGRef2VAURL
			}
			if err := s.sg.Cancel(endpoint, job.RemoteID); err != nil {
				s.queue.Log(id, "warn", "通知推理节点中止失败: "+err.Error())
			} else {
				s.queue.Log(id, "info", "已通知推理节点中止")
			}
		}
	}
	if err := s.queue.Cancel(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) retryJob(c *gin.Context) {
	job, err := s.queue.Retry(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) bumpJob(c *gin.Context) {
	job, err := s.queue.Bump(c.Param("id"), 1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) deleteJob(c *gin.Context) {
	id := c.Param("id")
	s.db.Where("job_id = ?", id).Delete(&models.JobEvent{})
	s.db.Where("job_id = ?", id).Delete(&models.JobAsset{})
	s.db.Delete(&models.Job{}, "id = ?", id)
	s.store.RemoveOutputs(id)
	_ = os.RemoveAll(s.store.JobMediaDir(id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) jobVideo(c *gin.Context) {
	s.serveOutput(c, false)
}

func (s *Server) jobImage(c *gin.Context) {
	s.serveOutput(c, true)
}

func (s *Server) serveOutput(c *gin.Context, image bool) {
	var job models.Job
	if err := s.db.First(&job, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	path := job.OutputPath
	if path == "" {
		if image {
			path = s.store.ImageOutputPath(job.ID)
		} else {
			path = s.store.OutputPath(job.ID)
		}
	}
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚无成品"})
		return
	}
	c.File(path)
}

func (s *Server) jobEvents(c *gin.Context) {
	var events []models.JobEvent
	s.db.Where("job_id = ?", c.Param("id")).Order("id asc").Find(&events)
	c.JSON(http.StatusOK, events)
}

func validMode(m string) bool {
	switch m {
	case models.ModeT2VA, models.ModeI2VA, models.ModeL2VA, models.ModeFL2VA, models.ModeRef2VA, models.ModeT2I, models.ModeI2I:
		return true
	}
	return false
}

func normalizeEngine(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", models.EngineH3, "local", "h3-base", "h3_base":
		return models.EngineH3
	case models.EngineFastH3, models.EngineH3Max, "fast-h3", "fast_h3", "h3max", "h3_max":
		return models.EngineFastH3
	case models.EngineLLadaImage, "llada", "llada_image", "lladaimage":
		return models.EngineLLadaImage
	default:
		return ""
	}
}

func countRole(assets []models.JobAsset, role string) int {
	n := 0
	for _, a := range assets {
		if a.Role == role {
			n++
		}
	}
	return n
}

func kindOf(mime, name string) string {
	m := strings.ToLower(mime)
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case strings.HasPrefix(m, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp":
		return "image"
	case strings.HasPrefix(m, "video/") || ext == ".mp4" || ext == ".webm" || ext == ".mov":
		return "video"
	case strings.HasPrefix(m, "audio/") || ext == ".mp3" || ext == ".wav" || ext == ".aac" || ext == ".m4a":
		return "audio"
	default:
		return "file"
	}
}

func clipRunes(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
