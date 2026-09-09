package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"aishow/internal/auth"
	"aishow/internal/models"
	"aishow/internal/settings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ctxUserKey = "auth_user"

type authBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) requireAuth(c *gin.Context) {
	user, err := auth.LookupSession(s.db, auth.SessionToken(c.Request))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	c.Set(ctxUserKey, *user)
	c.Next()
}

func currentUser(c *gin.Context) models.User {
	v, _ := c.Get(ctxUserKey)
	user, _ := v.(models.User)
	return user
}

func currentUserID(c *gin.Context) string {
	return currentUser(c).ID
}

func (s *Server) requireAdmin(c *gin.Context) {
	if !auth.IsAdmin(currentUser(c)) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	c.Next()
}

func (s *Server) authStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"has_users":      auth.HasUsers(s.db),
		"allow_register": settings.AllowRegister(s.db, s.cfg),
	})
}

func (s *Server) me(c *gin.Context) {
	c.JSON(http.StatusOK, currentUser(c))
}

func (s *Server) register(c *gin.Context) {
	var in authBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名和密码"})
		return
	}
	var user *models.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		first := !auth.HasUsers(tx)
		if !first && !settings.AllowRegister(tx, s.cfg) {
			return auth.ErrRegisterClosed
		}
		role := models.RoleUser
		if first {
			role = models.RoleAdmin
		}
		created, err := auth.CreateUser(tx, in.Username, in.Password, role)
		if err != nil {
			return err
		}
		if first {
			auth.ClaimOrphans(tx, created.ID)
		}
		user = created
		return nil
	})
	if err != nil {
		writeAuthError(c, err)
		return
	}
	if err := s.issueSession(c, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) login(c *gin.Context) {
	var in authBody
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名和密码"})
		return
	}
	user, err := auth.Authenticate(s.db, in.Username, in.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	if err := s.issueSession(c, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) logout(c *gin.Context) {
	auth.DestroySession(s.db, auth.SessionToken(c.Request))
	auth.ClearSessionCookie(c.Writer, c.Request)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) issueSession(c *gin.Context, userID string) error {
	sess, err := auth.CreateSession(s.db, userID, s.cfg.SessionDays)
	if err != nil {
		return err
	}
	auth.SetSessionCookie(c.Writer, c.Request, sess.ID, s.cfg.SessionDays)
	return nil
}

func (s *Server) changePassword(c *gin.Context) {
	var in struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写原密码和新密码"})
		return
	}
	user := currentUser(c)
	if !auth.CheckPassword(user.PasswordHash, in.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "原密码不正确"})
		return
	}
	if err := auth.SetPassword(s.db, &user, in.NewPassword); err != nil {
		writeAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) putRegisterPolicy(c *gin.Context) {
	var in struct {
		AllowRegister *bool `json:"allow_register"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.AllowRegister == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 allow_register"})
		return
	}
	if err := settings.SetAllowRegister(s.db, *in.AllowRegister); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"allow_register": *in.AllowRegister})
}

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, auth.ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, auth.ErrRegisterClosed):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, auth.ErrBadCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, auth.ErrLastAdmin), errors.Is(err, auth.ErrCannotDeleteSelf):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func eventVisibleTo(payload []byte, userID string) bool {
	var ev struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return false
	}
	switch ev.Type {
	case "queue.changed":
		return true
	case "job.created", "job.updated", "job.deleted":
		var job struct {
			UserID string `json:"user_id"`
		}
		if json.Unmarshal(ev.Data, &job) != nil {
			return false
		}
		return job.UserID == userID
	default:
		return false
	}
}
