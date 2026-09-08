package api

import (
	"net/http"
	"os"

	"aishow/internal/auth"
	"aishow/internal/models"
	"aishow/internal/settings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listUsers(c *gin.Context) {
	var users []models.User
	if err := s.db.Order("created_at asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	counts := map[string]int64{}
	type row struct {
		UserID string
		N      int64
	}
	var rows []row
	s.db.Model(&models.Job{}).Select("user_id as user_id, count(*) as n").Group("user_id").Scan(&rows)
	for _, r := range rows {
		counts[r.UserID] = r.N
	}
	out := make([]models.UserPublic, 0, len(users))
	for _, u := range users {
		out = append(out, models.UserPublic{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
			JobCount:  counts[u.ID],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"users":          out,
		"allow_register": settings.AllowRegister(s.db, s.cfg),
	})
}

func (s *Server) createManagedUser(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名和密码"})
		return
	}
	user, err := auth.CreateUser(s.db, in.Username, in.Password, in.Role)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.UserPublic{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

func (s *Server) patchUser(c *gin.Context) {
	var user models.User
	if err := s.db.First(&user, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var in struct {
		Password *string `json:"password"`
		Role     *string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Role != nil {
		next := auth.NormalizeRole(*in.Role)
		if user.Role == models.RoleAdmin && next != models.RoleAdmin && auth.AdminCount(s.db) <= 1 {
			writeAuthError(c, auth.ErrLastAdmin)
			return
		}
		if err := s.db.Model(&user).Update("role", next).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		user.Role = next
	}
	if in.Password != nil {
		if err := auth.SetPassword(s.db, &user, *in.Password); err != nil {
			writeAuthError(c, err)
			return
		}
		auth.DestroyUserSessions(s.db, user.ID)
	}
	c.JSON(http.StatusOK, models.UserPublic{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

func (s *Server) deleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == currentUserID(c) {
		writeAuthError(c, auth.ErrCannotDeleteSelf)
		return
	}
	var user models.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if user.Role == models.RoleAdmin && auth.AdminCount(s.db) <= 1 {
		writeAuthError(c, auth.ErrLastAdmin)
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var jobs []models.Job
		if err := tx.Where("user_id = ?", id).Find(&jobs).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(jobs))
		for _, job := range jobs {
			ids = append(ids, job.ID)
		}
		if len(ids) > 0 {
			if err := tx.Where("job_id IN ?", ids).Delete(&models.JobEvent{}).Error; err != nil {
				return err
			}
			if err := tx.Where("job_id IN ?", ids).Delete(&models.JobAsset{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", ids).Delete(&models.Job{}).Error; err != nil {
				return err
			}
		}
		var uploads []models.Upload
		if err := tx.Where("user_id = ?", id).Find(&uploads).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.Upload{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.User{}, "id = ?", id).Error; err != nil {
			return err
		}
		for _, job := range jobs {
			s.store.RemoveOutputs(job.ID)
			_ = os.RemoveAll(s.store.JobMediaDir(job.ID))
		}
		for _, up := range uploads {
			if up.Path != "" {
				_ = os.Remove(up.Path)
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
