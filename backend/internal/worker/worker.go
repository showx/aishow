package worker

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aishow/internal/config"
	"aishow/internal/llada"
	"aishow/internal/minimax"
	"aishow/internal/models"
	"aishow/internal/orchestrator"
	"aishow/internal/queue"
	"aishow/internal/settings"
	"aishow/internal/sglang"
	"aishow/internal/storage"

	"gorm.io/gorm"
)

type Worker struct {
	cfg        config.Config
	db         *gorm.DB
	queue      *queue.Service
	store      *storage.Store
	sglang     *sglang.Client
	minimax    *minimax.Client
	llada      *llada.Client
	orch       *orchestrator.Manager
	lastEngine string
}

func New(cfg config.Config, db *gorm.DB, q *queue.Service, store *storage.Store, orch *orchestrator.Manager) *Worker {
	return &Worker{
		cfg:     cfg,
		db:      db,
		queue:   q,
		store:   store,
		sglang:  sglang.New(),
		minimax: minimax.New(),
		llada:   llada.New(),
		orch:    orch,
	}
}

func (w *Worker) Start() {
	snap := settings.Snapshot(w.db, w.cfg)
	n := snap.WorkerConcurrency
	if n < 1 {
		n = 1
	}
	if autoSwitchOn(snap) && n > 1 {
		log.Printf("auto_switch_engine 已开启，单卡互斥，工位并发按 1 运行")
		n = 1
	}
	if w.orch != nil {
		w.lastEngine = w.orch.ActiveEngine(snap)
		if w.lastEngine != "" {
			w.orch.Remember(w.lastEngine)
		}
	}
	w.resumeActive()
	for i := 0; i < n; i++ {
		go w.loop(i+1, n)
	}
}

func (w *Worker) resumeActive() {
	var jobs []models.Job
	if err := w.db.Where("status = ?", models.StatusRunning).Find(&jobs).Error; err != nil {
		return
	}
	for i := range jobs {
		job := jobs[i]
		if w.lastEngine == "" {
			w.lastEngine = models.CanonicalEngine(job.Engine)
		}
		go func() {
			if job.RemoteID == "" {
				_ = w.queue.Update(&job, map[string]any{
					"status":   models.StatusQueued,
					"stage":    "服务重启，重新排队",
					"progress": 0,
				})
				return
			}
			snap := settings.Snapshot(w.db, w.cfg)
			w.queue.Log(job.ID, "info", "控制面重启，继续跟踪 "+job.RemoteID)
			var err error
			if isLLada(job.Engine) {
				err = w.waitLLada(&job, snap.LLaDAImageURL, job.RemoteID)
			} else if isFastH3(job.Engine) {
				err = w.waitRemote(&job, snap.FastH3URL, job.RemoteID)
			} else {
				err = w.waitRemote(&job, jobEndpoint(&job, snap), job.RemoteID)
			}
			if err != nil {
				if w.jobAborted(job.ID) {
					return
				}
				w.queue.Log(job.ID, "error", err.Error())
				_ = w.queue.Finish(&job, models.StatusFailed, "失败", err.Error(), "", 0, false)
			}
		}()
	}
}

func (w *Worker) loop(id, slots int) {
	for {
		job, err := w.queue.Claim(slots, w.lastEngine)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				time.Sleep(1200 * time.Millisecond)
			} else {
				time.Sleep(700 * time.Millisecond)
			}
			continue
		}
		w.queue.Log(job.ID, "info", fmt.Sprintf("工位 #%d 开始处理", id))
		if err := w.process(job); err != nil {
			if w.jobAborted(job.ID) {
				continue
			}
			w.queue.Log(job.ID, "error", err.Error())
			_ = w.queue.Finish(job, models.StatusFailed, "失败", err.Error(), "", 0, false)
			continue
		}
	}
}

func (w *Worker) jobAborted(id string) bool {
	var fresh models.Job
	if err := w.db.First(&fresh, "id = ?", id).Error; err != nil {
		return true
	}
	return fresh.Status == models.StatusCancelled
}

func (w *Worker) process(job *models.Job) error {
	snap := settings.Snapshot(w.db, w.cfg)
	mode := strings.ToLower(snap.InferenceMode)

	prompt := job.Prompt
	if job.EnhancePrompt && !isLLada(job.Engine) {
		if err := w.queue.Update(job, map[string]any{"stage": "提示增强", "progress": 12}); err != nil {
			return err
		}
		enhanced, err := w.enhance(job, snap)
		if err != nil {
			return fmt.Errorf("提示增强失败: %w", err)
		}
		prompt = enhanced
		_ = w.queue.Update(job, map[string]any{"enhanced_prompt": enhanced})
		w.queue.Log(job.ID, "info", "Context-IR 已生成结构化提示")
	}

	if err := w.queue.Update(job, map[string]any{"stage": "准备素材", "progress": 18}); err != nil {
		return err
	}
	conditions, err := w.prepareAssets(job, snap)
	if err != nil {
		return err
	}

	if mode != "mock" {
		if err := w.ensureEngine(job, snap); err != nil {
			return err
		}
	}
	w.captureTextUnderstanding(job, snap)

	if isLLada(job.Engine) {
		if mode == "mock" {
			return w.mock(job)
		}
		return w.processLLada(job, snap, prompt, conditions)
	}
	if isFastH3(job.Engine) {
		if mode == "mock" {
			return w.mock(job)
		}
		return w.processFastH3(job, snap, prompt)
	}
	if mode == "mock" {
		return w.mock(job)
	}

	endpoint := snap.SGLANGFL2VAURL
	task := job.Mode
	switch job.Mode {
	case models.ModeT2VA:
		task = "t2va"
	case models.ModeI2VA, models.ModeL2VA, models.ModeFL2VA:
		task = "fl2va"
	case models.ModeRef2VA:
		task = "ref2va"
		endpoint = snap.SGLANGRef2VAURL
	}
	if models.IsH3Turbo(job.Engine) {
		endpoint = snap.H3TurboURL
	}
	if models.IsH3PinkCherryInt8(job.Engine) {
		endpoint = snap.H3PinkCherryURL
	}
	if models.IsH3Ref2VAInt8(job.Engine) {
		task = "ref2va"
		endpoint = snap.SGLANGRef2VAURL
	}

	req := sglang.VideoRequest{
		Model:               "MiniMaxAI/MiniMax-H3",
		Prompt:              prompt,
		Seconds:             job.Duration,
		Task:                task,
		Conditions:          conditions,
		Quality:             job.Quality,
		NumOutputsPerPrompt: job.Outputs,
		NumInferenceSteps:   job.Steps,
		FlowShift:           job.FlowShift,
		AudioFlowShift:      job.AudioFlowShift,
		Seed:                job.Seed,
		Target: sglang.Target{
			ShortEdge:       job.ShortEdge,
			AspectRatio:     job.AspectRatio,
			DurationSeconds: job.Duration,
		},
	}

	if err := w.queue.Update(job, map[string]any{"stage": "提交推理", "progress": 22}); err != nil {
		return err
	}
	created, err := w.sglang.Create(endpoint, req)
	if err != nil {
		if mode == "auto" {
			w.queue.Log(job.ID, "warn", "SGLang 不可用，且当前为 auto 模式: "+err.Error())
		}
		return fmt.Errorf("提交 SGLang 失败: %w", err)
	}
	w.mergeSidecarTextEncoder(job, created.TextEncoder, created.TextEncoderLabel)
	_ = w.queue.Update(job, map[string]any{"remote_id": created.ID, "stage": "推理采样", "progress": 28})
	w.queue.Log(job.ID, "info", "远程任务 "+created.ID)
	return w.waitRemote(job, endpoint, created.ID)
}

func (w *Worker) waitRemote(job *models.Job, endpoint, remoteID string) error {
	deadline := time.Now().Add(timeoutFor(job.Duration))
	for time.Now().Before(deadline) {
		if w.jobAborted(job.ID) {
			return fmt.Errorf("任务已取消")
		}
		st, err := w.sglang.Status(endpoint, remoteID)
		if err != nil {
			w.queue.Log(job.ID, "warn", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		status := strings.ToLower(st.Status)
		progress := mapRemoteProgress(st.Progress)
		switch status {
		case "completed", "succeeded", "success":
			return w.download(job, endpoint, remoteID)
		case "cancelled":
			_ = w.queue.Cancel(job.ID)
			return fmt.Errorf("任务已取消")
		case "failed", "error":
			msg := "推理失败"
			if st.Error != nil && st.Error.Message != "" {
				msg = st.Error.Message
			}
			return fmt.Errorf("%s", msg)
		default:
			elapsed := ""
			if job.StartedAt != nil {
				elapsed = " " + time.Since(*job.StartedAt).Truncate(time.Second).String()
			}
			_ = w.queue.Update(job, map[string]any{
				"stage":    remoteStage(status) + elapsed,
				"progress": progress,
			})
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("等待推理超时")
}

func (w *Worker) download(job *models.Job, endpoint, remoteID string) error {
	_ = w.queue.Update(job, map[string]any{"stage": "封装成片", "progress": 92})
	dest := w.store.OutputPath(job.ID)
	n, err := w.sglang.Download(endpoint, remoteID, dest)
	if err != nil {
		return fmt.Errorf("下载成片失败: %w", err)
	}
	w.queue.Log(job.ID, "info", fmt.Sprintf("成片 %.1f MB", float64(n)/1024/1024))
	return w.queue.Finish(job, models.StatusSucceeded, "已完成", "", dest, n, true)
}

func (w *Worker) processFastH3(job *models.Job, snap models.SettingsPayload, prompt string) error {
	if job.Mode != models.ModeT2VA {
		return fmt.Errorf("本地 FastH3 Preview 只蒸馏了文生（t2va），暂不支持 %s", job.Mode)
	}
	endpoint := strings.TrimSpace(snap.FastH3URL)
	if endpoint == "" {
		return fmt.Errorf("未配置 FastH3 节点地址")
	}
	width, height := fastH3Canvas(job.AspectRatio, job.ShortEdge)
	frames := alignH3Frames(job.Duration)
	seconds := int(job.Duration + 0.5)
	if seconds < 1 {
		seconds = 1
	}
	req := sglang.FastH3Request{
		Model:             "fasth3",
		Prompt:            prompt,
		Seconds:           seconds,
		Size:              fmt.Sprintf("%dx%d", width, height),
		NumFrames:         frames,
		Seed:              job.Seed,
		NumInferenceSteps: 4,
		GuidanceScale:     1.0,
	}
	if err := w.queue.Update(job, map[string]any{"stage": "提交 FastH3", "progress": 22}); err != nil {
		return err
	}
	created, err := w.sglang.CreateJSON(endpoint, req)
	if err != nil {
		return fmt.Errorf("提交 FastH3 失败: %w", err)
	}
	w.mergeSidecarTextEncoder(job, created.TextEncoder, created.TextEncoderLabel)
	_ = w.queue.Update(job, map[string]any{"remote_id": created.ID, "stage": "FastH3 4-step 采样", "progress": 28})
	w.queue.Log(job.ID, "info", fmt.Sprintf("FastH3 任务 %s · %dx%d · %d 帧", created.ID, width, height, frames))
	return w.waitRemote(job, endpoint, created.ID)
}

func (w *Worker) processLLada(job *models.Job, snap models.SettingsPayload, prompt string, conditions []sglang.Condition) error {
	endpoint := snap.LLaDAImageURL
	if strings.TrimSpace(endpoint) == "" {
		return fmt.Errorf("未配置 LLaDA-Image 节点地址")
	}
	refs := make([]llada.Condition, 0, len(conditions))
	for _, c := range conditions {
		refs = append(refs, llada.Condition{Type: c.Type, URI: c.URI, Role: c.Role})
	}
	task := job.Mode
	if task != models.ModeI2I {
		task = models.ModeT2I
	}
	req := llada.ImageRequest{
		Prompt:            prompt,
		Task:              task,
		Conditions:        refs,
		Quality:           job.Quality,
		NumInferenceSteps: job.Steps,
		GuidanceScale:     lladaGuidance(job.Quality, job.FlowShift),
		Seed:              job.Seed,
		Target: llada.Target{
			ShortEdge:   job.ShortEdge,
			AspectRatio: job.AspectRatio,
		},
	}
	if err := w.queue.Update(job, map[string]any{"stage": "提交 LLaDA-Image", "progress": 22}); err != nil {
		return err
	}
	created, err := w.llada.Create(endpoint, req)
	if err != nil {
		return fmt.Errorf("提交 LLaDA-Image 失败: %w", err)
	}
	w.mergeSidecarTextEncoder(job, created.TextEncoder, created.TextEncoderLabel)
	_ = w.queue.Update(job, map[string]any{"remote_id": created.ID, "stage": "扩散采样", "progress": 28})
	w.queue.Log(job.ID, "info", "LLaDA-Image 任务 "+created.ID)
	return w.waitLLada(job, endpoint, created.ID)
}

func (w *Worker) waitLLada(job *models.Job, endpoint, remoteID string) error {
	deadline := time.Now().Add(45 * time.Minute)
	for time.Now().Before(deadline) {
		if w.jobAborted(job.ID) {
			_ = w.llada.Cancel(endpoint, remoteID)
			return fmt.Errorf("任务已取消")
		}
		st, err := w.llada.Status(endpoint, remoteID)
		if err != nil {
			w.queue.Log(job.ID, "warn", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		status := strings.ToLower(st.Status)
		progress := mapRemoteProgress(st.Progress)
		switch status {
		case "completed", "succeeded", "success":
			return w.downloadImage(job, endpoint, remoteID)
		case "cancelled":
			_ = w.queue.Cancel(job.ID)
			return fmt.Errorf("任务已取消")
		case "failed", "error":
			msg := "LLaDA-Image 推理失败"
			if st.Error != nil && st.Error.Message != "" {
				msg = st.Error.Message
			}
			return fmt.Errorf("%s", msg)
		default:
			elapsed := ""
			if job.StartedAt != nil {
				elapsed = " " + time.Since(*job.StartedAt).Truncate(time.Second).String()
			}
			_ = w.queue.Update(job, map[string]any{
				"stage":    remoteStage(status) + elapsed,
				"progress": progress,
			})
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return fmt.Errorf("等待 LLaDA-Image 超时")
}

func (w *Worker) downloadImage(job *models.Job, endpoint, remoteID string) error {
	_ = w.queue.Update(job, map[string]any{"stage": "写出图片", "progress": 92})
	dest := w.store.ImageOutputPath(job.ID)
	n, err := w.llada.Download(endpoint, remoteID, dest)
	if err != nil {
		return fmt.Errorf("下载图片失败: %w", err)
	}
	w.queue.Log(job.ID, "info", fmt.Sprintf("图片 %.1f MB", float64(n)/1024/1024))
	return w.queue.FinishMedia(job, models.StatusSucceeded, "已完成", "", dest, n, false, true)
}

func lladaGuidance(quality string, flowShift float64) float64 {
	if flowShift > 0 {
		return flowShift
	}
	if strings.EqualFold(quality, "base") {
		return 5
	}
	return 1
}

func isLLada(engine string) bool {
	return models.IsImageEngine(engine)
}

func isFastH3(engine string) bool {
	return models.IsFastH3(engine)
}

func fastH3Canvas(aspect string, short int) (int, int) {
	if short >= 640 {
		short = 768
	} else {
		short = 480
	}
	short = short / 32 * 32
	if short < 32 {
		short = 32
	}
	round := func(n int) int { return n / 32 * 32 }
	switch aspect {
	case "9:16":
		return short, round(short * 16 / 9)
	case "1:1":
		return short, short
	case "4:3":
		return round(short * 4 / 3), short
	case "3:4":
		return short, round(short * 4 / 3)
	case "21:9":
		return round(short * 21 / 9), short
	default:
		return round(short * 16 / 9), short
	}
}

func alignH3Frames(seconds float64) int {
	n := int(seconds*24 + 0.5)
	if n < 1 {
		n = 1
	}
	for n%17 != 5 {
		n++
	}
	if n > 345 {
		n = 345
	}
	return n
}

func (w *Worker) mock(job *models.Job) error {
	stages := []struct {
		name string
		pct  int
		wait time.Duration
	}{
		{"编译提示", 30, 700 * time.Millisecond},
		{"扩散采样", 58, 1100 * time.Millisecond},
		{"音画对齐", 78, 800 * time.Millisecond},
		{"封装成片", 94, 500 * time.Millisecond},
	}
	for _, st := range stages {
		if w.jobAborted(job.ID) {
			return fmt.Errorf("任务已取消")
		}
		_ = w.queue.Update(job, map[string]any{"stage": st.name, "progress": st.pct})
		w.queue.Log(job.ID, "info", "模拟阶段: "+st.name)
		time.Sleep(st.wait)
	}
	return w.queue.Finish(job, models.StatusSucceeded, "模拟完成", "", "", 0, false)
}

func (w *Worker) ensureEngine(job *models.Job, snap models.SettingsPayload) error {
	if w.orch == nil {
		return nil
	}
	if !autoSwitchOn(snap) {
		return nil
	}
	if err := w.queue.Update(job, map[string]any{"stage": "准备引擎", "progress": 8}); err != nil {
		return err
	}
	err := w.orch.EnsureReady(job.Engine, snap, func() bool {
		return w.jobAborted(job.ID)
	}, func(msg string) {
		w.queue.Log(job.ID, "info", msg)
		_ = w.queue.Update(job, map[string]any{"stage": msg, "progress": 10})
	})
	if err != nil {
		return fmt.Errorf("切换引擎失败: %w", err)
	}
	w.lastEngine = models.CanonicalEngine(job.Engine)
	return nil
}

func autoSwitchOn(snap models.SettingsPayload) bool {
	return snap.AutoSwitchEngine == nil || *snap.AutoSwitchEngine
}

func (w *Worker) enhance(job *models.Job, snap models.SettingsPayload) (string, error) {
	token := settings.Get(w.db, settings.KeyMiniMaxToken, w.cfg.MiniMaxAPIToken)
	if token == "" {
		return "", fmt.Errorf("未配置 MiniMax API Token，无法调用 H3-Context-IR")
	}
	ratio := job.AspectRatio
	if ratio == "auto" {
		ratio = "adaptive"
	}
	parts := []minimax.ContentPart{{Type: "text", Text: job.Prompt}}
	body := minimax.ContextIRRequest{
		Model:    "MiniMax-H3",
		Content:  parts,
		Duration: job.Duration,
		Ratio:    ratio,
	}
	id, err := w.minimax.CreateContextIR(snap.MiniMaxAPIBase, token, body)
	if err != nil {
		return "", err
	}
	return w.minimax.WaitPrompt(snap.MiniMaxAPIBase, token, id, 8*time.Minute)
}

func (w *Worker) prepareAssets(job *models.Job, snap models.SettingsPayload) ([]sglang.Condition, error) {
	if len(job.Assets) == 0 {
		return []sglang.Condition{}, nil
	}
	mediaDir := w.store.JobMediaDir(job.ID)
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return nil, err
	}
	var out []sglang.Condition
	for i, a := range job.Assets {
		var up models.Upload
		if err := w.db.First(&up, "id = ?", a.UploadID).Error; err != nil {
			return nil, fmt.Errorf("素材不存在: %s", a.UploadID)
		}
		name := fmt.Sprintf("%02d%s", i+1, filepath.Ext(up.Path))
		dest := filepath.Join(mediaDir, name)
		if err := w.store.Copy(up.Path, dest); err != nil {
			return nil, err
		}
		uri := w.store.ToFileURI(dest, snap.MediaFilePrefix)
		if snap.URIMode == "http" {
			uri = w.store.ToHTTPURI(snap.PublicBaseURL, job.ID, name)
		}
		_ = w.db.Model(&models.JobAsset{}).Where("id = ?", a.ID).Update("uri", uri).Error
		cond := sglang.Condition{
			Type:             a.Type,
			URI:              uri,
			Role:             a.Role,
			FrameIndex:       a.FrameIndex,
			StartTimeSeconds: a.StartSeconds,
		}
		out = append(out, cond)
	}
	return out, nil
}

func (w *Worker) captureTextUnderstanding(job *models.Job, snap models.SettingsPayload) {
	encoder, label := job.TextEncoder, job.TextEncoderLabel
	info := w.sglang.Inspect(jobEndpoint(job, snap))
	if info.TextEncoder != "" || info.TextEncoderLabel != "" {
		encoder, label = info.TextEncoder, info.TextEncoderLabel
	}
	w.applyTextUnderstanding(job, encoder, label, true)
}

func (w *Worker) mergeSidecarTextEncoder(job *models.Job, encoder, label string) {
	if strings.TrimSpace(encoder) == "" && strings.TrimSpace(label) == "" {
		return
	}
	w.applyTextUnderstanding(job, encoder, label, false)
}

func (w *Worker) applyTextUnderstanding(job *models.Job, encoder, label string, logIt bool) {
	id, lab := models.DescribeTextEncoder(job.Engine, encoder)
	if strings.TrimSpace(label) != "" {
		lab = strings.TrimSpace(label)
	}
	if strings.TrimSpace(encoder) != "" {
		id = models.TextEncoderBasename(encoder)
	}
	rewriter := models.PromptRewriterFor(job.Engine, job.EnhancePrompt)
	_ = w.queue.Update(job, map[string]any{
		"text_encoder":       id,
		"text_encoder_label": lab,
		"prompt_rewriter":    rewriter,
	})
	if !logIt {
		return
	}
	msg := "文字理解: " + lab
	if id != "" && id != lab {
		msg += " · " + id
	}
	if rewriter != "" {
		msg += " · 提示改写: " + rewriter
	} else if !models.IsImageEngine(job.Engine) {
		msg += " · 未开提示改写"
	}
	w.queue.Log(job.ID, "info", msg)
}

func jobEndpoint(job *models.Job, snap models.SettingsPayload) string {
	if isLLada(job.Engine) {
		return snap.LLaDAImageURL
	}
	if isFastH3(job.Engine) {
		return snap.FastH3URL
	}
	if models.IsH3Turbo(job.Engine) {
		return snap.H3TurboURL
	}
	if models.IsH3PinkCherryInt8(job.Engine) {
		return snap.H3PinkCherryURL
	}
	if models.IsH3Ref2VAInt8(job.Engine) || job.Mode == models.ModeRef2VA {
		return snap.SGLANGRef2VAURL
	}
	return snap.SGLANGFL2VAURL
}

func timeoutFor(seconds float64) time.Duration {
	d := time.Duration(seconds*12) * time.Minute
	if d < 2*time.Hour {
		d = 2 * time.Hour
	}
	if d > 4*time.Hour {
		d = 4 * time.Hour
	}
	return d
}

func mapRemoteProgress(p float64) int {
	if p <= 0 {
		return 20
	}
	if p <= 1 {
		p = p * 100
	}
	if p < 8 {
		p = 8
	}
	if p > 90 {
		p = 90
	}
	return int(p)
}

func remoteStage(status string) string {
	switch status {
	case "queued":
		return "推理排队"
	case "in_progress", "running", "processing":
		return "推理采样"
	default:
		return "推理中"
	}
}
