package queue

import (
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

func (s *Service) Claim() (*models.Job, error) {
	var job models.Job
	if err := s.db.Where("status = ?", models.StatusQueued).
		Order("priority desc, created_at asc").
		First(&job).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	res := s.db.Model(&models.Job{}).
		Where("id = ? AND status = ?", job.ID, models.StatusQueued).
		Updates(map[string]any{
			"status":     models.StatusRunning,
			"stage":      "认领任务",
			"progress":   4,
			"started_at": now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if err := s.db.Preload("Assets").First(&job, "id = ?", job.ID).Error; err != nil {
		return nil, err
	}
	s.hub.Broadcast("job.updated", job)
	s.hub.Broadcast("queue.changed", nil)
	return &job, nil
}

func (s *Service) Update(job *models.Job, fields map[string]any) error {
	if err := s.db.Model(job).Updates(fields).Error; err != nil {
		return err
	}
	_ = s.db.Preload("Assets").First(job, "id = ?", job.ID).Error
	s.hub.Broadcast("job.updated", job)
	return nil
}

func (s *Service) Log(jobID, level, message string) {
	ev := models.JobEvent{JobID: jobID, Level: level, Message: message, CreatedAt: time.Now()}
	_ = s.db.Create(&ev).Error
	s.hub.Broadcast("job.log", ev)
}

func (s *Service) Finish(job *models.Job, status, stage, errMsg string, outputPath string, outputSize int64, hasVideo bool) error {
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

func (s *Service) RecoverOrphans() error {
	return s.db.Model(&models.Job{}).
		Where("status = ?", models.StatusRunning).
		Updates(map[string]any{
			"status": models.StatusQueued,
			"stage":  "服务重启，重新排队",
			"progress": 0,
		}).Error
}
