package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"aishow/internal/config"
	"aishow/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const CookieName = "aishow_session"

var (
	ErrInvalidUsername = errors.New("用户名需 2–32 个字符，仅字母、数字、下划线与中文")
	ErrInvalidPassword = errors.New("密码至少 6 位")
	ErrUsernameTaken   = errors.New("用户名已被占用")
	ErrBadCredentials  = errors.New("用户名或密码不正确")
	ErrRegisterClosed  = errors.New("当前已关闭注册")
	ErrLastAdmin       = errors.New("至少保留一名管理员")
	ErrCannotDeleteSelf = errors.New("不能删除当前登录的账号")
)

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func NormalizeUsername(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 32 {
		return "", ErrInvalidUsername
	}
	for _, r := range name {
		ok := r == '_' || r == '-' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			(r >= 0x4e00 && r <= 0x9fff)
		if !ok {
			return "", ErrInvalidUsername
		}
	}
	return name, nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < 6 || len(password) > 72 {
		return ErrInvalidPassword
	}
	return nil
}

func NewSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, days int) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   days * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func SessionToken(r *http.Request) string {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return c.Value
	}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func HasUsers(db *gorm.DB) bool {
	var n int64
	db.Model(&models.User{}).Count(&n)
	return n > 0
}

func FindByUsername(db *gorm.DB, username string) (*models.User, error) {
	var user models.User
	err := db.Where("lower(username) = ?", strings.ToLower(username)).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func CreateUser(db *gorm.DB, username, password, role string) (*models.User, error) {
	name, err := NormalizeUsername(username)
	if err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if _, err := FindByUsername(db, name); err == nil {
		return nil, ErrUsernameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := models.User{
		ID:           uuid.NewString(),
		Username:     name,
		PasswordHash: hash,
		Role:         NormalizeRole(role),
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return &user, nil
}

func Authenticate(db *gorm.DB, username, password string) (*models.User, error) {
	name := strings.TrimSpace(username)
	if name == "" || password == "" {
		return nil, ErrBadCredentials
	}
	user, err := FindByUsername(db, name)
	if err != nil {
		return nil, ErrBadCredentials
	}
	if !CheckPassword(user.PasswordHash, password) {
		return nil, ErrBadCredentials
	}
	return user, nil
}

func CreateSession(db *gorm.DB, userID string, days int) (*models.Session, error) {
	token, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	if days < 1 {
		days = 30
	}
	_ = db.Where("expires_at < ?", time.Now()).Delete(&models.Session{}).Error
	sess := models.Session{
		ID:        token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := db.Create(&sess).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}

func LookupSession(db *gorm.DB, token string) (*models.User, error) {
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var sess models.Session
	if err := db.Where("id = ? AND expires_at > ?", token, time.Now()).First(&sess).Error; err != nil {
		return nil, err
	}
	var user models.User
	if err := db.First(&user, "id = ?", sess.UserID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func DestroySession(db *gorm.DB, token string) {
	if token == "" {
		return
	}
	db.Delete(&models.Session{}, "id = ?", token)
}

func NormalizeRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), models.RoleAdmin) {
		return models.RoleAdmin
	}
	return models.RoleUser
}

func IsAdmin(user models.User) bool {
	return user.Role == models.RoleAdmin
}

func AdminCount(db *gorm.DB) int64 {
	var n int64
	db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&n)
	return n
}

func EnsureAdmin(db *gorm.DB) error {
	if AdminCount(db) > 0 {
		return nil
	}
	var first models.User
	if err := db.Order("created_at asc").First(&first).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return db.Model(&first).Update("role", models.RoleAdmin).Error
}

func SetPassword(db *gorm.DB, user *models.User, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := db.Model(user).Update("password_hash", hash).Error; err != nil {
		return err
	}
	user.PasswordHash = hash
	return nil
}

func DestroyUserSessions(db *gorm.DB, userID string) {
	db.Delete(&models.Session{}, "user_id = ?", userID)
}

func ClaimOrphans(db *gorm.DB, userID string) {
	db.Model(&models.Job{}).Where("user_id = '' OR user_id IS NULL").Update("user_id", userID)
	db.Model(&models.Upload{}).Where("user_id = '' OR user_id IS NULL").Update("user_id", userID)
}

func Bootstrap(db *gorm.DB, cfg config.Config) error {
	user := strings.TrimSpace(cfg.AdminUser)
	pass := cfg.AdminPassword
	if user == "" || pass == "" {
		return nil
	}
	if existing, err := FindByUsername(db, user); err == nil {
		_ = existing
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	created, err := CreateUser(db, user, pass, models.RoleAdmin)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	ClaimOrphans(db, created.ID)
	return EnsureAdmin(db)
}
