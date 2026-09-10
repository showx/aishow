package queue

import (
	"strings"
	"time"

	"aishow/internal/hub"
	"aishow/internal/models"

	"gorm.io/gorm"
)

type Service struct {
	db  *gorm.DB
	hub *hub.Hub
}

func New(db *gorm.DB, h *hub.Hub) *Service {
	return &Service{db: db, hub: h}
}

func (s *Service) Enqueue(job *models.Job) error {
	if err := s.db.Create(job).Error; err != nil {
		return err
	}
	s.hub.Broadcast("job.created", job)
	s.hub.Broadcast("queue.changed", nil)
	return nil
}

func (s *Service) Claim(maxRunning int, preferEngine string) (*models.Job, error) {
	if maxRunning < 1 {
		maxRunning = 1
	}
	var job models.Job
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var running int64
		if err := tx.Model(&models.Job{}).Where("status = ?", models.StatusRunning).Count(&running).Error; err != nil {
			return err
		}
		if running >= int64(maxRunning) {
			return gorm.ErrRecordNotFound
		}
		q := tx.Where("status = ?", models.StatusQueued).Order(preferClaimOrder(preferEngine))
		if err := q.First(&job).Error; err != nil {
			return err
		}
		now := time.Now()
		res := tx.Model(&models.Job{}).
			Where("id = ? AND status = ?", job.ID, models.StatusQueued).
			Updates(map[string]any{
				"status":     models.StatusRunning,
				"stage":      "认领任务",
				"progress":   4,
				"started_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Preload("Assets").First(&job, "id = ?", job.ID).Error
	})
	if err != nil {
		return nil, err
	}
	s.hub.Broadcast("job.updated", job)
	s.hub.Broadcast("queue.changed", nil)
	return &job, nil
}

func (s *Service) Update(job *models.Job, fields map[string]any) error {
	res := s.db.Model(job).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	if err := s.db.Preload("Assets").First(job, "id = ?", job.ID).Error; err != nil {
		return nil
	}
	s.hub.Broadcast("job.updated", job)
	return nil
}

func (s *Service) Log(jobID, level, message string) {
	ev := models.JobEvent{JobID: jobID, Level: level, Message: message, CreatedAt: time.Now()}
	_ = s.db.Create(&ev).Error
	s.hub.Broadcast("job.log", ev)
}

func (s *Service) Finish(job *models.Job, status, stage, errMsg string, outputPath string, outputSize int64, hasVideo bool) error {
	return s.FinishMedia(job, status, stage, errMsg, outputPath, outputSize, hasVideo, false)
}

func (s *Service) FinishMedia(job *models.Job, status, stage, errMsg string, outputPath string, outputSize int64, hasVideo, hasImage bool) error {
	now := time.Now()
	fields := map[string]any{
		"status":        status,
		"stage":         stage,
		"error_message": errMsg,
		"finished_at":   &now,
	}
	if status == models.StatusSucceeded {
		fields["progress"] = 100
		fields["output_path"] = outputPath
		fields["output_size"] = outputSize
		fields["has_video"] = hasVideo
		fields["has_image"] = hasImage
	}
	if err := s.Update(job, fields); err != nil {
		return err
	}
	s.hub.Broadcast("queue.changed", nil)
	return nil
}

func (s *Service) Cancel(id string) error {
	var job models.Job
	if err := s.db.First(&job, "id = ?", id).Error; err != nil {
		return err
	}
	if job.Status != models.StatusQueued && job.Status != models.StatusRunning {
		return nil
	}
	now := time.Now()
	return s.Update(&job, map[string]any{
		"status":      models.StatusCancelled,
		"stage":       "已取消",
		"finished_at": &now,
	})
}

func (s *Service) Retry(id string) (*models.Job, error) {
	var job models.Job
	if err := s.db.Preload("Assets").First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := s.Update(&job, map[string]any{
		"status":        models.StatusQueued,
		"stage":         "重新排队",
		"progress":      0,
		"error_message": "",
		"remote_id":     "",
		"started_at":    nil,
		"finished_at":   nil,
		"has_video":     false,
		"has_image":     false,
	}); err != nil {
		return nil, err
	}
	s.hub.Broadcast("queue.changed", nil)
	return &job, nil
}

func (s *Service) Bump(id string, delta int) (*models.Job, error) {
	var job models.Job
	if err := s.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	job.Priority += delta
	if err := s.Update(&job, map[string]any{"priority": job.Priority}); err != nil {
		return nil, err
	}
	s.hub.Broadcast("queue.changed", nil)
	return &job, nil
}

func (s *Service) Position(job models.Job) int {
	if job.Status != models.StatusQueued {
		return 0
	}
	var n int64
	s.db.Model(&models.Job{}).
		Where("status = ? AND (priority > ? OR (priority = ? AND created_at < ?))",
			models.StatusQueued, job.Priority, job.Priority, job.CreatedAt).
		Count(&n)
	return int(n) + 1
}

func preferClaimOrder(preferEngine string) string {
	aliases := models.EngineAliases(preferEngine)
	if preferEngine == "" || len(aliases) == 0 {
		return "priority desc, created_at asc"
	}
	quoted := make([]string, 0, len(aliases))
	for _, a := range aliases {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || strings.ContainsAny(a, "'\";") {
			continue
		}
		quoted = append(quoted, "'"+a+"'")
	}
	if len(quoted) == 0 {
		return "priority desc, created_at asc"
	}
	return "priority desc, CASE WHEN lower(engine) IN (" + strings.Join(quoted, ",") + ") THEN 0 ELSE 1 END, created_at asc"
}

func (s *Service) RecoverOrphans() error {
	return s.db.Model(&models.Job{}).
		Where("status = ?", models.StatusRunning).
		Updates(map[string]any{
			"status": models.StatusQueued,
			"stage":  "服务重启，重新排队",
			"progress": 0,
		}).Error
}
