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

	"aishow/internal/chat"
	"aishow/internal/config"
	"aishow/internal/hub"
	"aishow/internal/llada"
	"aishow/internal/metrics"
	"aishow/internal/models"
	"aishow/internal/orchestrator"
	"aishow/internal/queue"
	"aishow/internal/settings"
	"aishow/internal/sglang"
	"aishow/internal/storage"
	"aishow/internal/tts"

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
	orch  *orchestrator.Manager
	chat  *chat.Client
	tts   *tts.Client
}

func New(cfg config.Config, db *gorm.DB, q *queue.Service, store *storage.Store, h *hub.Hub, hw *metrics.Collector, orch *orchestrator.Manager) *Server {
	return &Server{cfg: cfg, db: db, queue: q, store: store, hub: h, sg: sglang.New(), ll: llada.New(), hw: hw, orch: orch, chat: chat.New(), tts: tts.New()}
}

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  s.cfg.AllowCORSOrigin,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	{
		api.GET("/health", s.health)
		api.GET("/auth/status", s.authStatus)
		api.POST("/auth/login", s.login)
		api.POST("/auth/register", s.register)
		api.POST("/auth/logout", s.logout)
		// 推理边车按 HTTP URI 回源拉素材，不能走浏览器会话。
		api.GET("/media/:jobId/:name", s.mediaFile)

		authed := api.Group("")
		authed.Use(s.requireAuth)
		{
			authed.GET("/auth/me", s.me)
			authed.POST("/auth/password", s.changePassword)
			authed.GET("/system", s.system)
			authed.GET("/metrics", s.metrics)
			authed.GET("/settings", s.getSettings)
			authed.PUT("/settings", s.putSettings)
			authed.GET("/events", s.events)

			authed.POST("/uploads", s.createUpload)
			authed.GET("/uploads/:id", s.getUpload)
			authed.GET("/uploads/:id/raw", s.rawUpload)

			authed.POST("/jobs", s.createJob)
			authed.GET("/jobs", s.listJobs)
			authed.GET("/jobs/:id", s.getJob)
			authed.POST("/jobs/:id/cancel", s.cancelJob)
			authed.POST("/jobs/:id/retry", s.retryJob)
			authed.POST("/jobs/:id/bump", s.bumpJob)
			authed.DELETE("/jobs/:id", s.deleteJob)
			authed.GET("/jobs/:id/video", s.jobVideo)
			authed.GET("/jobs/:id/image", s.jobImage)
			authed.GET("/jobs/:id/events", s.jobEvents)

			authed.GET("/drama-projects", s.listDramaProjects)
			authed.POST("/drama-projects", s.createDramaProject)
			authed.GET("/drama-projects/:id", s.getDramaProject)
			authed.PATCH("/drama-projects/:id", s.patchDramaProject)
			authed.DELETE("/drama-projects/:id", s.deleteDramaProject)
			authed.POST("/drama-projects/:id/storyboard", s.runDramaStoryboard)
			authed.POST("/drama-projects/:id/write", s.runDramaWrite)
			authed.POST("/drama-projects/:id/shots/:index/rewrite", s.rewriteDramaShot)
			authed.GET("/chat/models", s.listChatModels)
			authed.POST("/drama-projects/:id/images", s.runDramaImages)
			authed.POST("/drama-projects/:id/images/:index/retry", s.retryDramaImage)
			authed.POST("/drama-projects/:id/videos", s.runDramaVideos)
			authed.POST("/drama-projects/:id/videos/:index/retry", s.retryDramaVideo)
			authed.POST("/drama-projects/:id/compile", s.runDramaCompile)
			authed.GET("/drama-projects/:id/video", s.dramaCompileVideo)

			admin := authed.Group("")
			admin.Use(s.requireAdmin)
			{
				admin.GET("/users", s.listUsers)
				admin.POST("/users", s.createManagedUser)
				admin.PATCH("/users/:id", s.patchUser)
				admin.DELETE("/users/:id", s.deleteUser)
				admin.PUT("/auth/register-policy", s.putRegisterPolicy)
			}
		}
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
	uid := currentUserID(c)
	st := models.SystemStatus{
		App:           "Aishow",
		Version:       "0.1.0",
		InferenceMode: snap.InferenceMode,
		WorkerSlots:   snap.WorkerConcurrency,
		Time:          time.Now(),
	}
	s.db.Model(&models.Job{}).Where("user_id = ? AND status = ?", uid, models.StatusQueued).Count(&st.QueueDepth)
	s.db.Model(&models.Job{}).Where("user_id = ? AND status = ?", uid, models.StatusRunning).Count(&st.Running)
	s.db.Model(&models.Job{}).Where("user_id = ? AND status = ? AND finished_at >= ?", uid, models.StatusSucceeded, today).Count(&st.SucceededToday)
	s.db.Model(&models.Job{}).Where("user_id = ? AND status = ? AND finished_at >= ?", uid, models.StatusFailed, today).Count(&st.FailedToday)
	s.db.Model(&models.Job{}).Where("user_id = ?", uid).Count(&st.TotalJobs)

	inspect := func(url string) models.SidecarHealth {
		if s.orch != nil {
			return s.orch.Inspect(url)
		}
		return s.sg.Inspect(url)
	}
	fl := inspect(snap.SGLANGFL2VAURL)
	tb := inspect(snap.H3TurboURL)
	pc := inspect(snap.H3PinkCherryURL)
	rf := inspect(snap.SGLANGRef2VAURL)
	dr := inspect(snap.H3DirectorURL)
	fh := inspect(snap.FastH3URL)
	ll := inspect(snap.LLaDAImageURL)
	ch := s.chat.Inspect(snap.ChatURL, settings.ChatToken(s.db, s.cfg))
	tt := s.tts.Inspect(snap.TTSURL, settings.TTSToken(s.db, s.cfg))
	endpoint := func(name, url string, h models.SidecarHealth) models.EndpointHealth {
		return models.EndpointHealth{
			Name:             name,
			URL:              url,
			Healthy:          h.Ready,
			LatencyMS:        h.LatencyMS,
			Detail:           h.Detail,
			TextEncoder:      h.TextEncoder,
			TextEncoderLabel: h.TextEncoderLabel,
		}
	}
	st.Endpoints = []models.EndpointHealth{
		endpoint("H3-Base FL2VA", snap.SGLANGFL2VAURL, fl),
		endpoint("H3 Turbo LoRA", snap.H3TurboURL, tb),
		endpoint("H3 PinkCherry INT8", snap.H3PinkCherryURL, pc),
		endpoint("H3 Ref2VA INT8", snap.SGLANGRef2VAURL, rf),
		endpoint("H3 Timeline Director", snap.H3DirectorURL, dr),
		endpoint("FastH3 · GGUF", snap.FastH3URL, fh),
		endpoint("LLaDA-Image", snap.LLaDAImageURL, ll),
		endpoint("本地 Chat", snap.ChatURL, ch),
		endpoint("本地 TTS", snap.TTSURL, tt),
	}
	st.Hardware = s.hw.Snapshot()
	if snap.AutoSwitchEngine != nil {
		st.AutoSwitchEngine = *snap.AutoSwitchEngine
	}
	st.MaxLoadedEngines = 1
	if snap.MaxLoadedEngines != nil && *snap.MaxLoadedEngines > 1 {
		st.MaxLoadedEngines = *snap.MaxLoadedEngines
	}
	if s.orch != nil {
		st.ActiveEngine = s.orch.ActiveEngine(snap)
	}
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
			if !eventVisibleTo(payload, currentUserID(c)) {
				continue
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
		UserID:    currentUserID(c),
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
	up, ok := s.ownedUpload(c, c.Param("id"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, up)
}

func (s *Server) rawUpload(c *gin.Context) {
	up, ok := s.ownedUpload(c, c.Param("id"))
	if !ok {
		return
	}
	c.File(up.Path)
}

func (s *Server) mediaFile(c *gin.Context) {
	path := filepath.Join(s.store.JobMediaDir(c.Param("jobId")), filepath.Base(c.Param("name")))
	c.File(path)
}

func (s *Server) buildAssets(jobID, userID, mode string, conds []models.AssetCondition) ([]models.JobAsset, error) {
	var assets []models.JobAsset
	for _, cnd := range conds {
		if cnd.UploadID == "" {
			continue
		}
		var up models.Upload
		if err := s.db.First(&up, "id = ? AND user_id = ?", cnd.UploadID, userID).Error; err != nil {
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
	q := s.db.Preload("Assets").Where("user_id = ?", currentUserID(c)).Order("created_at desc").Limit(limit)
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
	job, ok := s.ownedJob(c, true)
	if !ok {
		return
	}
	job.QueuePosition = s.queue.Position(*job)
	c.JSON(http.StatusOK, job)
}

func (s *Server) stopRemote(job *models.Job) {
	if job.RemoteID == "" {
		return
	}
	snap := settings.Snapshot(s.db, s.cfg)
	if models.IsImageEngine(job.Engine) {
		if err := s.ll.Cancel(snap.LLaDAImageURL, job.RemoteID); err != nil {
			s.queue.Log(job.ID, "warn", "通知 LLaDA-Image 中止失败: "+err.Error())
		} else {
			s.queue.Log(job.ID, "info", "已通知 LLaDA-Image 中止")
		}
		return
	}
	if models.IsFastH3(job.Engine) {
		if err := s.sg.Cancel(snap.FastH3URL, job.RemoteID); err != nil {
			s.queue.Log(job.ID, "warn", "通知 FastH3 中止失败: "+err.Error())
		} else {
			s.queue.Log(job.ID, "info", "已通知 FastH3 中止")
		}
		return
	}
	endpoint := snap.SGLANGFL2VAURL
	if models.IsH3Turbo(job.Engine) {
		endpoint = snap.H3TurboURL
	}
	if models.IsH3PinkCherryInt8(job.Engine) {
		endpoint = snap.H3PinkCherryURL
	}
	if models.IsH3Director(job.Engine) {
		endpoint = snap.H3DirectorURL
	}
	if models.IsH3Ref2VAInt8(job.Engine) || (job.Mode == models.ModeRef2VA && !models.IsH3Director(job.Engine)) {
		endpoint = snap.SGLANGRef2VAURL
	}
	if err := s.sg.Cancel(endpoint, job.RemoteID); err != nil {
		s.queue.Log(job.ID, "warn", "通知推理节点中止失败: "+err.Error())
	} else {
		s.queue.Log(job.ID, "info", "已通知推理节点中止")
	}
}

func (s *Server) cancelJob(c *gin.Context) {
	job, ok := s.ownedJob(c, false)
	if !ok {
		return
	}
	s.stopRemote(job)
	if err := s.queue.Cancel(job.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) retryJob(c *gin.Context) {
	if _, ok := s.ownedJob(c, false); !ok {
		return
	}
	job, err := s.queue.Retry(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) bumpJob(c *gin.Context) {
	if _, ok := s.ownedJob(c, false); !ok {
		return
	}
	job, err := s.queue.Bump(c.Param("id"), 1)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) deleteJob(c *gin.Context) {
	job, ok := s.ownedJob(c, false)
	if !ok {
		return
	}
	id := job.ID
	if job.Status == models.StatusQueued || job.Status == models.StatusRunning {
		s.stopRemote(job)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", id).Delete(&models.JobEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("job_id = ?", id).Delete(&models.JobAsset{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Job{}, "id = ?", id).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.store.RemoveOutputs(id)
	_ = os.RemoveAll(s.store.JobMediaDir(id))
	s.hub.Broadcast("job.deleted", gin.H{"id": id, "user_id": job.UserID})
	s.hub.Broadcast("queue.changed", nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) jobVideo(c *gin.Context) {
	s.serveOutput(c, false)
}

func (s *Server) jobImage(c *gin.Context) {
	s.serveOutput(c, true)
}

func (s *Server) serveOutput(c *gin.Context, image bool) {
	job, ok := s.ownedJob(c, false)
	if !ok {
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
	if _, ok := s.ownedJob(c, false); !ok {
		return
	}
	var events []models.JobEvent
	s.db.Where("job_id = ?", c.Param("id")).Order("id asc").Find(&events)
	c.JSON(http.StatusOK, events)
}

func (s *Server) ownedJob(c *gin.Context, withDetails bool) (*models.Job, bool) {
	q := s.db
	if withDetails {
		q = q.Preload("Assets").Preload("Events")
	}
	var job models.Job
	if err := q.First(&job, "id = ? AND user_id = ?", c.Param("id"), currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return nil, false
	}
	return &job, true
}

func (s *Server) ownedUpload(c *gin.Context, id string) (*models.Upload, bool) {
	var up models.Upload
	if err := s.db.First(&up, "id = ? AND user_id = ?", id, currentUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return nil, false
	}
	return &up, true
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
	case models.EngineH3Turbo, "h3-turbo-lora", "turbo-lora", "h3_turbo", "h3turbo":
		return models.EngineH3Turbo
	case models.EngineH3Ref2VAInt8, "h3-ref2va", "ref2va-int8", "h3_ref2va_int8":
		return models.EngineH3Ref2VAInt8
	case models.EngineH3PinkCherryInt8, "pinkcherry", "pinkcherry-int8", "h3-pinkcherry", "h3_pinkcherry_int8":
		return models.EngineH3PinkCherryInt8
	case models.EngineH3Director, "h3-timeline-director", "timeline-director", "director":
		return models.EngineH3Director
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
