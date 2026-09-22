package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aishow/internal/drama"
	"aishow/internal/models"
	"aishow/internal/settings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var dramaLocks sync.Map

func lockDrama(id string) func() {
	v, _ := dramaLocks.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Server) listDramaProjects(c *gin.Context) {
	q := s.db.Where("user_id = ?", currentUserID(c)).Order("updated_at desc")
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("title LIKE ? OR idea LIKE ?", like, like)
	}
	var items []models.DramaProject
	if err := q.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (s *Server) createDramaProject(c *gin.Context) {
	var body struct {
		Title      string `json:"title"`
		Idea       string `json:"idea"`
		Style      string `json:"style"`
		StyleNotes string `json:"style_notes"`
		TargetSec  int    `json:"target_sec"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写题材"})
		return
	}
	idea := strings.TrimSpace(body.Idea)
	if idea == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写题材或意图"})
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = drama.ClipRunes(idea, 24)
	}
	target := body.TargetSec
	if target <= 0 {
		target = 60
	}
	if target < 10 {
		target = 10
	}
	if target > 180 {
		target = 180
	}
	p := models.DramaProject{
		ID:             uuid.NewString(),
		UserID:         currentUserID(c),
		Title:          title,
		Idea:           idea,
		Style:          strings.TrimSpace(body.Style),
		StyleNotes:     strings.TrimSpace(body.StyleNotes),
		TargetSec:      target,
		Step:           models.DramaStepWrite,
		Status:         models.DramaStatusDraft,
		ShotsJSON:      "[]",
		ImageRefsJSON:  "[]",
		ImageEngine:    models.EngineQwenImage,
		ImageAspect:    "9:16",
		ImageShortEdge: 1024,
		ImageQuality:   "base",
		VideoEngine:    models.EngineH3,
		ContinueEngine: models.EngineH3Ref2VAInt8,
		VideoAspect:    "9:16",
		VideoShortEdge: 480,
		VideoDuration:  5,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.db.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) getDramaProject(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) patchDramaProject(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	unlock := lockDrama(p.ID)
	defer unlock()
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	var body struct {
		Title          *string                `json:"title"`
		Idea           *string                `json:"idea"`
		Style          *string                `json:"style"`
		StyleNotes     *string                `json:"style_notes"`
		TargetSec      *int                   `json:"target_sec"`
		Step           *string                `json:"step"`
		ScriptText     *string                `json:"script_text"`
		Shots          []models.DramaShot     `json:"shots"`
		ImageEngine    *string                `json:"image_engine"`
		ImageAspect    *string                `json:"image_aspect"`
		ImageShortEdge *int                   `json:"image_short_edge"`
		ImageQuality   *string                `json:"image_quality"`
		VideoEngine    *string                `json:"video_engine"`
		ContinueEngine *string                `json:"continue_engine"`
		VideoAspect    *string                `json:"video_aspect"`
		VideoShortEdge *int                   `json:"video_short_edge"`
		VideoDuration  *float64               `json:"video_duration"`
		ImageRefs      []models.DramaImageRef `json:"image_refs"`
		BurnSubtitles  *bool                  `json:"burn_subtitles"`
		MixTTS         *bool                  `json:"mix_tts"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]any{"updated_at": time.Now()}
	if body.Title != nil {
		updates["title"] = strings.TrimSpace(*body.Title)
	}
	if body.Idea != nil {
		updates["idea"] = strings.TrimSpace(*body.Idea)
	}
	if body.Style != nil {
		updates["style"] = strings.TrimSpace(*body.Style)
	}
	if body.StyleNotes != nil {
		updates["style_notes"] = strings.TrimSpace(*body.StyleNotes)
	}
	if body.TargetSec != nil {
		n := *body.TargetSec
		if n < 10 {
			n = 10
		}
		if n > 180 {
			n = 180
		}
		updates["target_sec"] = n
	}
	if body.ScriptText != nil {
		updates["script_text"] = *body.ScriptText
		p.ScriptText = *body.ScriptText
	}
	if body.Shots != nil {
		updates["shots_json"] = drama.EncodeShots(body.Shots)
		p.ShotsJSON = updates["shots_json"].(string)
	}
	if body.ImageEngine != nil {
		eng := normalizeEngine(*body.ImageEngine)
		if eng == "" || !models.IsImageEngine(eng) {
			writeErr(c, badRequest("出图引擎请选择 LLaDA-Image 或 Qwen-Image-2.1"))
			return
		}
		updates["image_engine"] = eng
	}
	if body.ImageAspect != nil {
		updates["image_aspect"] = strings.TrimSpace(*body.ImageAspect)
	}
	if body.ImageShortEdge != nil {
		updates["image_short_edge"] = *body.ImageShortEdge
	}
	if body.ImageQuality != nil {
		updates["image_quality"] = strings.TrimSpace(*body.ImageQuality)
	}
	if body.VideoEngine != nil {
		eng := normalizeEngine(*body.VideoEngine)
		if eng == "" || models.IsImageEngine(eng) {
			writeErr(c, badRequest("不支持的成片引擎"))
			return
		}
		updates["video_engine"] = eng
	}
	if body.ContinueEngine != nil {
		eng := normalizeEngine(*body.ContinueEngine)
		if eng == "" || models.IsImageEngine(eng) {
			writeErr(c, badRequest("不支持的续写引擎"))
			return
		}
		updates["continue_engine"] = eng
	}
	if body.VideoAspect != nil {
		updates["video_aspect"] = strings.TrimSpace(*body.VideoAspect)
	}
	if body.VideoShortEdge != nil {
		updates["video_short_edge"] = *body.VideoShortEdge
	}
	if body.VideoDuration != nil {
		d := *body.VideoDuration
		if d < 2 {
			d = 2
		}
		if d > 30 {
			d = 30
		}
		updates["video_duration"] = d
	}
	if body.ImageRefs != nil {
		updates["image_refs_json"] = drama.EncodeImageRefs(body.ImageRefs)
		p.ImageRefsJSON = updates["image_refs_json"].(string)
	}
	if body.BurnSubtitles != nil {
		updates["burn_subtitles"] = *body.BurnSubtitles
	}
	if body.MixTTS != nil {
		updates["mix_tts"] = *body.MixTTS
	}
	if body.Step != nil {
		step := strings.TrimSpace(*body.Step)
		s.hydrateDrama(p)
		shots := drama.ParseShots(p.ShotsJSON)
		if err := drama.RequireStep(step, p.ScriptText, shots, p.Shots); err != nil {
			writeErr(c, badRequest(err.Error()))
			return
		}
		updates["step"] = step
	}
	if err := s.db.Model(p).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) deleteDramaProject(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := s.db.Delete(&models.DramaProject{}, "id = ?", p.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) runDramaStoryboard(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	if err := drama.RequireScript(p.ScriptText); err != nil {
		writeErr(c, badRequest(err.Error()))
		return
	}
	var body struct {
		Count   int    `json:"count"`
		Replace bool   `json:"replace"`
		Mode    string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&body)
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "generate" {
		mode = "llm"
	}
	unlock := lockDrama(p.ID)
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		unlock()
		writeErr(c, err)
		return
	}
	existing := drama.ParseShots(p.ShotsJSON)
	if mode != "llm" {
		if len(existing) > 0 && !body.Replace {
			unlock()
			writeErr(c, badRequest("已有分镜，请手动编辑；若要重新拆空镜，请勾选覆盖"))
			return
		}
		n := body.Count
		if n <= 0 {
			n = drama.SuggestedShotCount(p.TargetSec)
		}
		shots := drama.FillEmptyShots(n)
		if err := s.db.Model(p).Updates(map[string]any{
			"shots_json": drama.EncodeShots(shots),
			"step":       models.DramaStepStoryboard,
			"status":     models.DramaStatusDraft,
			"updated_at": time.Now(),
		}).Error; err != nil {
			unlock()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		unlock()
		p.ShotsJSON = drama.EncodeShots(shots)
		p.Step = models.DramaStepStoryboard
		s.hydrateDrama(p)
		c.JSON(http.StatusOK, p)
		return
	}
	if p.Status == models.DramaStatusStoryboard {
		unlock()
		writeErr(c, badRequest("正在拆分镜，请稍候"))
		return
	}
	if len(existing) > 0 && !body.Replace {
		unlock()
		writeErr(c, badRequest("已有分镜。本地拆分镜会覆盖，请确认后重试"))
		return
	}
	if err := s.db.Model(p).Updates(map[string]any{
		"status":            models.DramaStatusStoryboard,
		"storyboard_result": "",
		"step":              models.DramaStepStoryboard,
		"updated_at":        time.Now(),
	}).Error; err != nil {
		unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.Status = models.DramaStatusStoryboard
	p.Step = models.DramaStepStoryboard
	style, notes, script := p.Style, p.StyleNotes, p.ScriptText
	unlock()
	go s.generateDramaStoryboard(p.ID, style, notes, script, p.TargetSec)
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) generateDramaStoryboard(projectID, style, styleNotes, script string, targetSec int) {
	n := drama.SuggestedShotCount(targetSec)
	system := drama.StoryboardSystemPrompt(n, targetSec)
	user := drama.StoryboardUserPrompt(style, styleNotes, script, n, targetSec)
	mock := func() string { return drama.EncodeShots(drama.MockStoryboard(script, style, n)) }
	content, model, err := s.dramaChat(true, system, user, mock)
	shots, parseErr := []models.DramaShot(nil), error(nil)
	if err == nil {
		shots, parseErr = drama.ParseStoryboardContent(content)
	}
	if err != nil || parseErr != nil {
		log.Printf("drama storyboard retry without json_object project=%s: %v %v", projectID, err, parseErr)
		content, model, err = s.dramaChat(false, system, user, mock)
		if err == nil {
			shots, parseErr = drama.ParseStoryboardContent(content)
		}
	}
	unlock := lockDrama(projectID)
	defer unlock()
	updates := map[string]any{"updated_at": time.Now(), "step": models.DramaStepStoryboard}
	if err != nil {
		log.Printf("drama storyboard project=%s: %v", projectID, err)
		updates["status"] = models.DramaStatusFailed
		updates["storyboard_result"] = err.Error()
		_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
		return
	}
	if parseErr != nil {
		log.Printf("drama storyboard parse project=%s: %v", projectID, parseErr)
		updates["status"] = models.DramaStatusFailed
		updates["storyboard_result"] = parseErr.Error()
		_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
		return
	}
	updates["shots_json"] = drama.EncodeShots(shots)
	updates["storyboard_model"] = model
	updates["storyboard_result"] = ""
	updates["status"] = models.DramaStatusDraft
	_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
}

func (s *Server) runDramaImages(c *gin.Context) {
	s.runDramaMedia(c, models.DramaKindImage, false)
}

func (s *Server) retryDramaImage(c *gin.Context) {
	s.runDramaMedia(c, models.DramaKindImage, true)
}

func (s *Server) runDramaVideos(c *gin.Context) {
	s.runDramaMedia(c, models.DramaKindVideo, false)
}

func (s *Server) retryDramaVideo(c *gin.Context) {
	s.runDramaMedia(c, models.DramaKindVideo, true)
}

func (s *Server) runDramaMedia(c *gin.Context, kind string, single bool) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	unlock := lockDrama(p.ID)
	defer unlock()
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	shots := drama.ParseShots(p.ShotsJSON)
	step := models.DramaStepImage
	if kind == models.DramaKindVideo {
		step = models.DramaStepVideo
	}
	if err := drama.RequireStep(step, p.ScriptText, shots, p.Shots); err != nil {
		writeErr(c, badRequest(err.Error()))
		return
	}
	var body struct {
		Indexes []int `json:"indexes"`
		Queue   *bool `json:"queue"`
	}
	_ = c.ShouldBindJSON(&body)
	queueNext := true
	if body.Queue != nil {
		queueNext = *body.Queue
	}
	indexes := body.Indexes
	if single {
		idx, err := strconv.Atoi(c.Param("index"))
		if err != nil || idx < 1 || idx > len(shots) {
			writeErr(c, badRequest("无效的镜头序号"))
			return
		}
		indexes = []int{idx}
		queueNext = false
	}
	if len(indexes) == 0 {
		for _, sh := range shots {
			indexes = append(indexes, sh.Index)
		}
	}
	n, err := s.queueDramaMedia(p, shots, indexes, kind, queueNext, single)
	if err != nil {
		writeErr(c, err)
		return
	}
	if n == 0 {
		writeErr(c, badRequest("没有可提交的镜头"))
		return
	}
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	c.JSON(http.StatusOK, p)
}

func (s *Server) queueDramaMedia(p *models.DramaProject, shots []models.DramaShot, indexes []int, kind string, queueNext, replace bool) (int, error) {
	want := map[int]bool{}
	for _, idx := range indexes {
		if idx < 1 || idx > len(shots) {
			return 0, badRequest("无效的镜头序号")
		}
		want[idx] = true
	}
	started := 0
	status := models.DramaStatusImaging
	if kind == models.DramaKindVideo {
		status = models.DramaStatusVideoing
	}
	for i := range shots {
		if !want[shots[i].Index] {
			continue
		}
		if kind == models.DramaKindImage {
			if !replace && imageOccupied(shots[i], p.Shots) {
				continue
			}
			if replace {
				shots[i].ImageJobID = ""
				shots[i].ImageUploadID = ""
				shots[i].QueueImage = false
			}
			if started == 0 && s.canStartDramaImage(shots, i) {
				if _, err := s.startDramaShotImage(p, shots, i); err != nil {
					return started, err
				}
				started++
				continue
			}
			if queueNext {
				shots[i].QueueImage = true
				started++
			}
			continue
		}
		if !replace && videoOccupied(shots[i], p.Shots) {
			continue
		}
		if replace {
			shots[i].VideoJobID = ""
			shots[i].VideoUploadID = ""
			shots[i].QueueVideo = false
		}
		if started == 0 && s.canStartDramaVideo(shots, i) {
			if _, err := s.startDramaShotVideo(p, shots, i); err != nil {
				return started, err
			}
			started++
			continue
		}
		if queueNext {
			shots[i].QueueVideo = true
			started++
		}
	}
	if started == 0 {
		return 0, nil
	}
	step := models.DramaStepImage
	if kind == models.DramaKindVideo {
		step = models.DramaStepVideo
	}
	if err := s.saveDramaShots(p, shots, map[string]any{
		"status": status,
		"step":   step,
	}); err != nil {
		return started, err
	}
	return started, nil
}

func imageOccupied(shot models.DramaShot, views []models.DramaShotView) bool {
	if shot.QueueImage {
		return true
	}
	if shot.ImageJobID == "" {
		return false
	}
	for _, v := range views {
		if v.Index == shot.Index && v.ImageJob != nil {
			return drama.JobBusy(v.ImageJob.Status) || drama.JobSucceeded(v.ImageJob)
		}
	}
	return true
}

func videoOccupied(shot models.DramaShot, views []models.DramaShotView) bool {
	if shot.QueueVideo {
		return true
	}
	if shot.VideoJobID == "" {
		return false
	}
	for _, v := range views {
		if v.Index == shot.Index && v.VideoJob != nil {
			return drama.JobBusy(v.VideoJob.Status) || drama.JobSucceeded(v.VideoJob)
		}
	}
	return true
}

func (s *Server) canStartDramaImage(shots []models.DramaShot, i int) bool {
	if i <= 0 || shots[i].SkipPrevImages {
		return true
	}
	prev := shots[i-1]
	if prev.ImageJobID == "" && !prev.QueueImage {
		return true
	}
	var job models.Job
	if prev.ImageJobID == "" {
		return false
	}
	if err := s.db.First(&job, "id = ?", prev.ImageJobID).Error; err != nil {
		return false
	}
	return job.Status == models.StatusSucceeded
}

func (s *Server) canStartDramaVideo(shots []models.DramaShot, i int) bool {
	if i <= 0 || !shots[i].ContinueFromPrev {
		return true
	}
	prev := shots[i-1]
	if prev.VideoJobID == "" {
		return false
	}
	var job models.Job
	if err := s.db.First(&job, "id = ?", prev.VideoJobID).Error; err != nil {
		return false
	}
	return job.Status == models.StatusSucceeded
}

func (s *Server) startDramaShotImage(p *models.DramaProject, shots []models.DramaShot, i int) (*models.Job, error) {
	shot := shots[i]
	base := firstNonEmpty(shot.ImagePrompt, shot.Scene)
	if strings.TrimSpace(base) == "" {
		return nil, badRequest(fmt.Sprintf("第 %d 镜缺少出图提示或画面说明", shot.Index))
	}
	if i > 0 && !shot.SkipPrevImages && shots[i-1].ImageUploadID == "" && shots[i-1].ImageJobID != "" {
		up, err := s.registerJobOutputUpload(p.UserID, shots[i-1].ImageJobID, "image")
		if err != nil {
			return nil, err
		}
		if up != nil {
			shots[i-1].ImageUploadID = up.ID
		}
	}
	var prev *models.DramaShot
	if i > 0 {
		prev = &shots[i-1]
	}
	refs := drama.MergeImageRefs(p, shot, prev)
	refID := drama.FirstImageRefID(refs)
	engine := emptyAs(p.ImageEngine, models.EngineQwenImage)
	mode := models.ModeT2I
	var conds []models.AssetCondition
	if models.IsQwenImage(engine) {
		for _, r := range refs {
			if strings.TrimSpace(r.UploadID) == "" {
				continue
			}
			conds = append(conds, models.AssetCondition{UploadID: r.UploadID, Role: "reference", Type: "image"})
			if len(conds) >= 10 {
				break
			}
		}
		if len(conds) > 0 {
			mode = models.ModeI2I
		}
	} else if refID != "" {
		mode = models.ModeI2I
		conds = append(conds, models.AssetCondition{UploadID: refID, Role: "reference", Type: "image"})
	}
	prompt := drama.ImagePrompt(p, base, refID != "")
	if note := drama.FormatImageRefs(refs); note != "" {
		prompt = strings.TrimSpace(prompt + "\n" + note)
	}
	quality := emptyAs(p.ImageQuality, "turbo")
	if models.IsQwenImage(engine) {
		quality = "base"
	}
	job, err := s.enqueueJob(p.UserID, models.CreateJobRequest{
		Title:          fmt.Sprintf("%s · 镜%02d 出图", p.Title, shot.Index),
		Mode:           mode,
		Engine:         engine,
		Prompt:         prompt,
		AspectRatio:    emptyAs(p.ImageAspect, "9:16"),
		ShortEdge:      p.ImageShortEdge,
		Quality:        quality,
		Conditions:     conds,
		DramaID:        p.ID,
		DramaShotIndex: shot.Index,
		DramaKind:      models.DramaKindImage,
	})
	if err != nil {
		return nil, err
	}
	shots[i].ImageJobID = job.ID
	shots[i].QueueImage = false
	return job, s.saveDramaShots(p, shots, nil)
}

func (s *Server) startDramaShotVideo(p *models.DramaProject, shots []models.DramaShot, i int) (*models.Job, error) {
	shot := shots[i]
	prompt := drama.VideoPrompt(shot, p.StyleNotes)
	if prompt == "" {
		return nil, badRequest(fmt.Sprintf("第 %d 镜缺少成片提示", shot.Index))
	}
	if shot.ImageUploadID == "" && shot.ImageJobID != "" {
		up, err := s.registerJobOutputUpload(p.UserID, shot.ImageJobID, "image")
		if err != nil {
			return nil, err
		}
		if up != nil {
			shots[i].ImageUploadID = up.ID
			shot.ImageUploadID = up.ID
		}
	}
	var prev *models.DramaShot
	if i > 0 {
		prev = &shots[i-1]
		if shot.ContinueFromPrev && prev.VideoUploadID == "" && prev.VideoJobID != "" {
			up, err := s.registerJobOutputUpload(p.UserID, prev.VideoJobID, "video")
			if err != nil {
				return nil, err
			}
			if up != nil {
				shots[i-1].VideoUploadID = up.ID
				prev.VideoUploadID = up.ID
			}
		}
		if shot.ContinueFromPrev && models.ContinueUsesLastFrame(emptyAs(p.ContinueEngine, models.EngineH3Ref2VAInt8)) {
			up, err := s.ensureLastFrameUpload(p.UserID, prev)
			if err != nil {
				log.Printf("drama last frame project=%s shot=%d: %v", p.ID, shot.Index, err)
			}
			if up != nil {
				shots[i-1].LastFrameUploadID = up.ID
				prev.LastFrameUploadID = up.ID
			}
		}
	}
	engine, mode, conds := planDramaVideo(p, shot, prev)
	if models.SupportsRef2VA(engine) {
		if note := drama.FormatVideoImageNote(drama.VideoImageRefs(p, shot, prev)); note != "" {
			prompt = strings.TrimSpace(prompt + "\n" + note)
		}
	}
	dur := shot.Duration
	if dur <= 0 {
		dur = p.VideoDuration
	}
	job, err := s.enqueueJob(p.UserID, models.CreateJobRequest{
		Title:          fmt.Sprintf("%s · 镜%02d 成片", p.Title, shot.Index),
		Mode:           mode,
		Engine:         engine,
		Prompt:         prompt,
		Duration:       dur,
		AspectRatio:    emptyAs(p.VideoAspect, "9:16"),
		ShortEdge:      p.VideoShortEdge,
		Conditions:     conds,
		DramaID:        p.ID,
		DramaShotIndex: shot.Index,
		DramaKind:      models.DramaKindVideo,
	})
	if err != nil {
		return nil, err
	}
	shots[i].VideoJobID = job.ID
	shots[i].QueueVideo = false
	return job, s.saveDramaShots(p, shots, nil)
}

func planDramaVideo(p *models.DramaProject, shot models.DramaShot, prev *models.DramaShot) (engine, mode string, conds []models.AssetCondition) {
	engine = emptyAs(p.VideoEngine, models.EngineH3)
	continueEngine := emptyAs(p.ContinueEngine, models.EngineH3Ref2VAInt8)
	zero := 0
	start := 0.0
	imgConds := drama.VideoImageConditions(p, shot, prev)
	if shot.ContinueFromPrev && prev != nil {
		engine = continueEngine
		if models.SupportsRef2VA(engine) {
			mode = models.ModeRef2VA
			if prev.VideoUploadID != "" {
				conds = append(conds, models.AssetCondition{UploadID: prev.VideoUploadID, Role: "reference", Type: "video", StartSeconds: &start})
			}
			conds = append(conds, imgConds...)
			if len(conds) == 0 {
				mode = models.ModeT2VA
				if models.IsH3Ref2VAInt8(engine) {
					engine = emptyAs(p.VideoEngine, models.EngineH3)
					if models.IsH3Ref2VAInt8(engine) {
						engine = models.EngineH3
					}
				}
			}
			return engine, mode, conds
		}
		if models.SupportsFirstFrame(engine) {
			last := ""
			if prev != nil {
				last = strings.TrimSpace(prev.LastFrameUploadID)
			}
			if last != "" && shot.ImageUploadID != "" {
				lastIdx := -1
				return engine, models.ModeFL2VA, []models.AssetCondition{
					{UploadID: last, Role: "keyframe", Type: "image", FrameIndex: &zero},
					{UploadID: shot.ImageUploadID, Role: "keyframe", Type: "image", FrameIndex: &lastIdx},
				}
			}
			if last != "" {
				return engine, models.ModeI2VA, []models.AssetCondition{{UploadID: last, Role: "keyframe", Type: "image", FrameIndex: &zero}}
			}
			if shot.ImageUploadID != "" {
				return engine, models.ModeI2VA, []models.AssetCondition{{UploadID: shot.ImageUploadID, Role: "keyframe", Type: "image", FrameIndex: &zero}}
			}
		}
		if models.IsFastH3(engine) {
			return engine, models.ModeT2VA, nil
		}
		if models.IsH3Ref2VAInt8(engine) {
			engine = models.EngineH3
		}
		if shot.ImageUploadID != "" && models.SupportsFirstFrame(engine) {
			return engine, models.ModeI2VA, []models.AssetCondition{{UploadID: shot.ImageUploadID, Role: "keyframe", Type: "image", FrameIndex: &zero}}
		}
		return engine, models.ModeT2VA, nil
	}
	if models.IsH3Ref2VAInt8(engine) {
		if len(imgConds) > 0 {
			return engine, models.ModeRef2VA, imgConds
		}
		return models.EngineH3, models.ModeT2VA, nil
	}
	if models.IsH3Director(engine) {
		if len(imgConds) > 0 {
			return engine, models.ModeRef2VA, imgConds
		}
		return engine, models.ModeT2VA, nil
	}
	if models.IsFastH3(engine) {
		return engine, models.ModeT2VA, nil
	}
	if shot.ImageUploadID != "" && models.SupportsFirstFrame(engine) {
		return engine, models.ModeI2VA, []models.AssetCondition{{UploadID: shot.ImageUploadID, Role: "keyframe", Type: "image", FrameIndex: &zero}}
	}
	return engine, models.ModeT2VA, nil
}

func (s *Server) runDramaCompile(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	unlock := lockDrama(p.ID)
	defer unlock()
	if err := s.db.First(p, "id = ?", p.ID).Error; err != nil {
		writeErr(c, err)
		return
	}
	s.hydrateDrama(p)
	if err := drama.RequireStep(models.DramaStepCompile, p.ScriptText, drama.ParseShots(p.ShotsJSON), p.Shots); err != nil {
		writeErr(c, badRequest(err.Error()))
		return
	}
	if p.CompileStatus == "compiling" || p.Status == models.DramaStatusCompiling {
		writeErr(c, badRequest("正在合成短片，请稍候"))
		return
	}
	var body struct {
		Indexes       []int `json:"indexes"`
		BurnSubtitles *bool `json:"burn_subtitles"`
		MixTTS        *bool `json:"mix_tts"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.BurnSubtitles != nil {
		p.BurnSubtitles = *body.BurnSubtitles
	}
	if body.MixTTS != nil {
		p.MixTTS = *body.MixTTS
	}
	want := map[int]bool{}
	for _, idx := range body.Indexes {
		want[idx] = true
	}
	var paths []string
	var picked []int
	var duration float64
	for _, sh := range p.Shots {
		if len(want) > 0 && !want[sh.Index] {
			continue
		}
		if sh.VideoJob == nil || sh.VideoJob.Status != models.StatusSucceeded {
			if len(want) > 0 {
				writeErr(c, badRequest(fmt.Sprintf("第 %d 镜还没有成片，无法合并", sh.Index)))
				return
			}
			continue
		}
		path := strings.TrimSpace(sh.VideoJob.OutputPath)
		if path == "" {
			path = s.store.OutputPath(sh.VideoJob.ID)
		}
		if _, err := os.Stat(path); err != nil {
			writeErr(c, badRequest(fmt.Sprintf("第 %d 镜没有真实成片文件（模拟任务无法拼接）", sh.Index)))
			return
		}
		paths = append(paths, path)
		picked = append(picked, sh.Index)
		duration += sh.Duration
	}
	if len(paths) == 0 {
		writeErr(c, badRequest("请勾选至少一段已生成的成片"))
		return
	}
	if p.MixTTS {
		snap := settings.Snapshot(s.db, s.cfg)
		if strings.TrimSpace(snap.TTSURL) == "" && !strings.EqualFold(snap.InferenceMode, "mock") {
			writeErr(c, badRequest("请先在推理节点填写本地 TTS 地址，或关掉「叠本地语音」"))
			return
		}
	}
	if err := s.db.Model(p).Updates(map[string]any{
		"compile_status":       "compiling",
		"compile_result":       "",
		"compile_indexes_json": drama.EncodeIndexes(picked),
		"status":               models.DramaStatusCompiling,
		"step":                 models.DramaStepCompile,
		"burn_subtitles":       p.BurnSubtitles,
		"mix_tts":              p.MixTTS,
		"updated_at":           time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.CompileStatus = "compiling"
	p.CompileIndexes = picked
	p.Status = models.DramaStatusCompiling
	p.Step = models.DramaStepCompile
	go s.compileDramaProject(compileDramaArgs{
		ProjectID:     p.ID,
		UserID:        p.UserID,
		Title:         p.Title,
		Idea:          p.Idea,
		Paths:         paths,
		Picked:        picked,
		Shots:         p.Shots,
		Aspect:        p.VideoAspect,
		ShortEdge:     p.VideoShortEdge,
		Duration:      duration,
		BurnSubtitles: p.BurnSubtitles,
		MixTTS:        p.MixTTS,
	})
	c.JSON(http.StatusOK, p)
}

type compileDramaArgs struct {
	ProjectID, UserID, Title, Idea string
	Paths                          []string
	Picked                         []int
	Shots                          []models.DramaShotView
	Aspect                         string
	ShortEdge                      int
	Duration                       float64
	BurnSubtitles, MixTTS          bool
}

func (s *Server) compileDramaProject(args compileDramaArgs) {
	projectID, userID, title, idea := args.ProjectID, args.UserID, args.Title, args.Idea
	paths, picked, aspect, shortEdge, duration := args.Paths, args.Picked, args.Aspect, args.ShortEdge, args.Duration
	w, h := drama.CompileSize(aspect, shortEdge)
	jobID := uuid.NewString()
	dest := s.store.OutputPath(jobID)
	opts := drama.ConcatOptions{Width: w, Height: h}
	if args.BurnSubtitles {
		opts.Cues = drama.BuildSubtitleCues(args.Shots, picked)
	}
	tmpDir := ""
	if args.MixTTS {
		var err error
		tmpDir, err = os.MkdirTemp("", "aishow-tts-*")
		if err == nil {
			defer os.RemoveAll(tmpDir)
			opts.ClipAudio = s.speakDramaClips(tmpDir, args.Shots, picked)
		}
	}
	cerr := drama.ConcatVideos(paths, dest, opts)

	unlock := lockDrama(projectID)
	defer unlock()
	updates := map[string]any{"updated_at": time.Now()}
	if cerr != nil {
		log.Printf("drama compile project=%s: %v", projectID, cerr)
		updates["compile_status"] = "failed"
		updates["compile_result"] = cerr.Error()
		updates["status"] = models.DramaStatusFailed
		_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
		return
	}
	st, _ := os.Stat(dest)
	size := int64(0)
	if st != nil {
		size = st.Size()
	}
	now := time.Now()
	job := models.Job{
		ID:          jobID,
		UserID:      userID,
		Title:       title + " · 全成短片",
		Mode:        models.ModeT2VA,
		Engine:      models.EngineH3,
		Status:      models.StatusSucceeded,
		Prompt:      drama.ClipRunes(idea, 80),
		Duration:    duration,
		AspectRatio: emptyAs(aspect, "9:16"),
		ShortEdge:   shortEdge,
		Progress:    100,
		Stage:       "已完成",
		OutputPath:  dest,
		OutputSize:  size,
		HasVideo:    true,
		DramaID:     projectID,
		DramaKind:   models.DramaKindCompile,
		CreatedAt:   now,
		UpdatedAt:   now,
		StartedAt:   &now,
		FinishedAt:  &now,
	}
	if err := s.db.Create(&job).Error; err != nil {
		updates["compile_status"] = "failed"
		updates["compile_result"] = err.Error()
		updates["status"] = models.DramaStatusFailed
		_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
		return
	}
	s.hub.Broadcast("job.created", job)
	updates["compile_status"] = "done"
	updates["compile_job_id"] = jobID
	updates["compile_result"] = ""
	updates["status"] = models.DramaStatusDone
	_ = s.db.Model(&models.DramaProject{}).Where("id = ?", projectID).Updates(updates).Error
}

func (s *Server) dramaCompileVideo(c *gin.Context) {
	p, err := s.loadDramaOwned(c, c.Param("id"))
	if err != nil {
		writeErr(c, err)
		return
	}
	if p.CompileJobID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚无合成短片"})
		return
	}
	var job models.Job
	if err := s.db.First(&job, "id = ? AND user_id = ?", p.CompileJobID, p.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚无合成短片"})
		return
	}
	path := job.OutputPath
	if path == "" {
		path = s.store.OutputPath(job.ID)
	}
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "尚无合成短片"})
		return
	}
	c.File(path)
}

func (s *Server) OnDramaJobFinished(job *models.Job) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("drama after finish panic: %v", rec)
		}
	}()
	if job == nil || job.DramaID == "" {
		return
	}
	unlock := lockDrama(job.DramaID)
	defer unlock()
	var p models.DramaProject
	if err := s.db.First(&p, "id = ?", job.DramaID).Error; err != nil {
		return
	}
	shots := drama.ParseShots(p.ShotsJSON)
	idx := job.DramaShotIndex - 1
	if idx < 0 || idx >= len(shots) {
		return
	}
	if job.DramaKind == models.DramaKindImage && shots[idx].ImageJobID == job.ID {
		shots[idx].QueueImage = false
		if job.Status == models.StatusSucceeded {
			if up, err := s.registerJobOutputUpload(p.UserID, job.ID, "image"); err == nil && up != nil {
				shots[idx].ImageUploadID = up.ID
			}
		}
	}
	if job.DramaKind == models.DramaKindVideo && shots[idx].VideoJobID == job.ID {
		shots[idx].QueueVideo = false
		if job.Status == models.StatusSucceeded {
			if up, err := s.registerJobOutputUpload(p.UserID, job.ID, "video"); err == nil && up != nil {
				shots[idx].VideoUploadID = up.ID
			}
		}
	}
	_ = s.saveDramaShots(&p, shots, nil)
	if job.Status == models.StatusSucceeded {
		if job.DramaKind == models.DramaKindImage {
			s.startDramaNextImage(&p, shots, idx)
		}
		if job.DramaKind == models.DramaKindVideo {
			s.startDramaNextVideo(&p, shots, idx)
		}
	}
	s.syncDramaStatus(&p, shots, job)
}

func (s *Server) startDramaNextImage(p *models.DramaProject, shots []models.DramaShot, from int) {
	for i := from + 1; i < len(shots); i++ {
		if !shots[i].QueueImage || shots[i].ImageJobID != "" {
			continue
		}
		if !s.canStartDramaImage(shots, i) {
			return
		}
		if _, err := s.startDramaShotImage(p, shots, i); err != nil {
			log.Printf("drama next image project=%s shot=%d: %v", p.ID, shots[i].Index, err)
		}
		return
	}
}

func (s *Server) startDramaNextVideo(p *models.DramaProject, shots []models.DramaShot, from int) {
	for i := from + 1; i < len(shots); i++ {
		if !shots[i].QueueVideo || shots[i].VideoJobID != "" {
			continue
		}
		if !s.canStartDramaVideo(shots, i) {
			return
		}
		if _, err := s.startDramaShotVideo(p, shots, i); err != nil {
			log.Printf("drama next video project=%s shot=%d: %v", p.ID, shots[i].Index, err)
		}
		return
	}
}

func (s *Server) syncDramaStatus(p *models.DramaProject, shots []models.DramaShot, job *models.Job) {
	s.hydrateDrama(p)
	status := models.DramaStatusDraft
	if job != nil && job.Status == models.StatusFailed {
		status = models.DramaStatusFailed
	}
	imageBusy, videoBusy := false, false
	imageDone, videoDone := 0, 0
	for _, v := range p.Shots {
		if v.QueueImage || (v.ImageJob != nil && drama.JobBusy(v.ImageJob.Status)) {
			imageBusy = true
		}
		if v.QueueVideo || (v.VideoJob != nil && drama.JobBusy(v.VideoJob.Status)) {
			videoBusy = true
		}
		if drama.JobSucceeded(v.ImageJob) {
			imageDone++
		}
		if drama.JobSucceeded(v.VideoJob) {
			videoDone++
		}
	}
	if imageBusy {
		status = models.DramaStatusImaging
	} else if videoBusy {
		status = models.DramaStatusVideoing
	} else if p.CompileStatus == "compiling" {
		status = models.DramaStatusCompiling
	} else if p.CompileStatus == "done" && videoDone > 0 {
		status = models.DramaStatusDone
	} else if job != nil && job.Status != models.StatusFailed {
		status = models.DramaStatusDraft
	}
	_ = s.db.Model(p).Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (s *Server) saveDramaShots(p *models.DramaProject, shots []models.DramaShot, extra map[string]any) error {
	fields := map[string]any{
		"shots_json": drama.EncodeShots(shots),
		"updated_at": time.Now(),
	}
	for k, v := range extra {
		fields[k] = v
	}
	if err := s.db.Model(&models.DramaProject{}).Where("id = ?", p.ID).Updates(fields).Error; err != nil {
		return err
	}
	p.ShotsJSON = fields["shots_json"].(string)
	return nil
}

func (s *Server) hydrateDrama(p *models.DramaProject) {
	shots := drama.ParseShots(p.ShotsJSON)
	ids := make([]string, 0, len(shots)*3)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	for _, sh := range shots {
		add(sh.ImageJobID)
		add(sh.VideoJobID)
		for _, extra := range sh.ExtraImageJobIDs {
			add(extra)
		}
	}
	add(p.CompileJobID)
	jobs := map[string]models.Job{}
	if len(ids) > 0 {
		var list []models.Job
		if err := s.db.Where("id IN ?", ids).Find(&list).Error; err == nil {
			for _, j := range list {
				jobs[j.ID] = j
			}
		}
	}
	views := make([]models.DramaShotView, 0, len(shots))
	for _, sh := range shots {
		view := models.DramaShotView{DramaShot: sh}
		if j, ok := jobs[sh.ImageJobID]; ok {
			cp := j
			view.ImageJob = &cp
			if j.HasImage {
				u := "/api/v1/jobs/" + j.ID + "/image"
				view.ImageURL = u
				view.ImageURLs = append(view.ImageURLs, u)
			}
		}
		for _, extraID := range sh.ExtraImageJobIDs {
			if j, ok := jobs[extraID]; ok {
				view.ExtraImageJobs = append(view.ExtraImageJobs, j)
				if j.HasImage {
					view.ImageURLs = append(view.ImageURLs, "/api/v1/jobs/"+j.ID+"/image")
				}
			}
		}
		if j, ok := jobs[sh.VideoJobID]; ok {
			cp := j
			view.VideoJob = &cp
			if j.HasVideo {
				view.VideoURL = "/api/v1/jobs/" + j.ID + "/video"
			}
		}
		if sh.LastFrameUploadID != "" {
			view.LastFrameURL = "/api/v1/uploads/" + sh.LastFrameUploadID + "/raw"
		}
		if len(sh.ImageRefs) > 0 {
			view.ImageRefViews = uploadRefViews(sh.ImageRefs)
		}
		views = append(views, view)
	}
	p.Shots = views
	p.ImageRefs = uploadRefViews(drama.ParseImageRefs(p.ImageRefsJSON))
	p.CompileIndexes = drama.ParseIndexes(p.CompileIndexesJSON)
	if p.CompileJobID != "" {
		if j, ok := jobs[p.CompileJobID]; ok && j.HasVideo {
			p.CompileURL = "/api/v1/jobs/" + j.ID + "/video"
		} else if p.CompileStatus == "done" {
			p.CompileURL = "/api/v1/drama-projects/" + p.ID + "/video"
		}
	}
}

func (s *Server) loadDramaOwned(c *gin.Context, id string) (*models.DramaProject, error) {
	var p models.DramaProject
	if err := s.db.First(&p, "id = ? AND user_id = ?", id, currentUserID(c)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, notFound("项目不存在")
		}
		return nil, err
	}
	return &p, nil
}

func uploadRefViews(refs []models.DramaImageRef) []models.DramaImageRefView {
	out := make([]models.DramaImageRefView, 0, len(refs))
	for _, r := range refs {
		out = append(out, models.DramaImageRefView{
			DramaImageRef: r,
			URL:           "/api/v1/uploads/" + r.UploadID + "/raw",
		})
	}
	return out
}

func (s *Server) ensureLastFrameUpload(userID string, shot *models.DramaShot) (*models.Upload, error) {
	if shot == nil {
		return nil, nil
	}
	if shot.LastFrameUploadID != "" {
		var up models.Upload
		if err := s.db.First(&up, "id = ? AND user_id = ?", shot.LastFrameUploadID, userID).Error; err == nil {
			if _, err := os.Stat(up.Path); err == nil {
				return &up, nil
			}
		}
	}
	src := s.resolveMediaPath(userID, shot.VideoUploadID, shot.VideoJobID, "video")
	if src == "" {
		return nil, nil
	}
	tmp := filepath.Join(os.TempDir(), "aishow-last-"+uuid.NewString()+".jpg")
	defer os.Remove(tmp)
	if err := drama.ExtractLastFrame(src, tmp); err != nil {
		return nil, err
	}
	return s.registerLocalUpload(userID, tmp, "last-frame.jpg", "image", "image/jpeg")
}

func (s *Server) resolveMediaPath(userID, uploadID, jobID, kind string) string {
	if uploadID != "" {
		var up models.Upload
		if err := s.db.First(&up, "id = ? AND user_id = ?", uploadID, userID).Error; err == nil {
			if _, err := os.Stat(up.Path); err == nil {
				return up.Path
			}
		}
	}
	if jobID == "" {
		return ""
	}
	var job models.Job
	if err := s.db.First(&job, "id = ? AND user_id = ?", jobID, userID).Error; err != nil {
		return ""
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
		return ""
	}
	return src
}

func (s *Server) speakDramaClips(tmpDir string, shots []models.DramaShotView, picked []int) []string {
	snap := settings.Snapshot(s.db, s.cfg)
	if strings.TrimSpace(snap.TTSURL) == "" {
		return nil
	}
	byIndex := map[int]models.DramaShotView{}
	for _, sh := range shots {
		byIndex[sh.Index] = sh
	}
	out := make([]string, len(picked))
	token := settings.TTSToken(s.db, s.cfg)
	for i, idx := range picked {
		sh := byIndex[idx]
		text := strings.TrimSpace(sh.Dialogue)
		if text == "" {
			continue
		}
		dest := filepath.Join(tmpDir, fmt.Sprintf("line-%02d.wav", idx))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := s.tts.Speak(ctx, snap.TTSURL, token, snap.TTSModel, snap.TTSVoice, text, dest)
		cancel()
		if err != nil {
			log.Printf("drama tts shot=%d: %v", idx, err)
			continue
		}
		out[i] = dest
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func emptyAs(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
