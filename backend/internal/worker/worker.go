package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aishow/internal/config"
	"aishow/internal/minimax"
	"aishow/internal/models"
	"aishow/internal/queue"
	"aishow/internal/settings"
	"aishow/internal/sglang"
	"aishow/internal/storage"

	"gorm.io/gorm"
)

type Worker struct {
	cfg     config.Config
	db      *gorm.DB
	queue   *queue.Service
	store   *storage.Store
	sglang  *sglang.Client
	minimax *minimax.Client
}

func New(cfg config.Config, db *gorm.DB, q *queue.Service, store *storage.Store) *Worker {
	return &Worker{
		cfg:     cfg,
		db:      db,
		queue:   q,
		store:   store,
		sglang:  sglang.New(),
		minimax: minimax.New(),
	}
}

func (w *Worker) Start() {
	n := settings.Snapshot(w.db, w.cfg).WorkerConcurrency
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		go w.loop(i + 1)
	}
	w.resumeActive()
}

func (w *Worker) resumeActive() {
	var jobs []models.Job
	if err := w.db.Where("status = ?", models.StatusRunning).Find(&jobs).Error; err != nil {
		return
	}
	for i := range jobs {
		job := jobs[i]
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
			endpoint := snap.SGLANGFL2VAURL
			if job.Mode == models.ModeRef2VA {
				endpoint = snap.SGLANGRef2VAURL
			}
			w.queue.Log(job.ID, "info", "控制面重启，继续跟踪 "+job.RemoteID)
			if err := w.waitRemote(&job, endpoint, job.RemoteID); err != nil {
				var fresh models.Job
				if w.db.First(&fresh, "id = ?", job.ID).Error == nil && fresh.Status == models.StatusCancelled {
					return
				}
				w.queue.Log(job.ID, "error", err.Error())
				_ = w.queue.Finish(&job, models.StatusFailed, "失败", err.Error(), "", 0, false)
			}
		}()
	}
}

func (w *Worker) loop(id int) {
	for {
		job, err := w.queue.Claim()
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
			var fresh models.Job
			if w.db.First(&fresh, "id = ?", job.ID).Error == nil && fresh.Status == models.StatusCancelled {
				continue
			}
			w.queue.Log(job.ID, "error", err.Error())
			_ = w.queue.Finish(job, models.StatusFailed, "失败", err.Error(), "", 0, false)
			continue
		}
	}
}

func (w *Worker) process(job *models.Job) error {
	snap := settings.Snapshot(w.db, w.cfg)
	mode := strings.ToLower(snap.InferenceMode)

	prompt := job.Prompt
	if job.EnhancePrompt {
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
	_ = w.queue.Update(job, map[string]any{"remote_id": created.ID, "stage": "推理采样", "progress": 28})
	w.queue.Log(job.ID, "info", "远程任务 "+created.ID)
	return w.waitRemote(job, endpoint, created.ID)
}

func (w *Worker) waitRemote(job *models.Job, endpoint, remoteID string) error {
	deadline := time.Now().Add(timeoutFor(job.Duration))
	for time.Now().Before(deadline) {
		var fresh models.Job
		if err := w.db.First(&fresh, "id = ?", job.ID).Error; err == nil && fresh.Status == models.StatusCancelled {
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
			} else if fresh.StartedAt != nil {
				elapsed = " " + time.Since(*fresh.StartedAt).Truncate(time.Second).String()
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
		var fresh models.Job
		if err := w.db.First(&fresh, "id = ?", job.ID).Error; err == nil && fresh.Status == models.StatusCancelled {
			return fmt.Errorf("任务已取消")
		}
		_ = w.queue.Update(job, map[string]any{"stage": st.name, "progress": st.pct})
		w.queue.Log(job.ID, "info", "模拟阶段: "+st.name)
		time.Sleep(st.wait)
	}
	return w.queue.Finish(job, models.StatusSucceeded, "模拟完成", "", "", 0, false)
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
