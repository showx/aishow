package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"aishow/internal/config"
	"aishow/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	conn, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := conn.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		return nil, err
	}
	if err := conn.Exec("PRAGMA busy_timeout=8000;").Error; err != nil {
		return nil, err
	}
	if err := conn.Exec("PRAGMA foreign_keys=ON;").Error; err != nil {
		return nil, err
	}
	if err := conn.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.Job{},
		&models.JobAsset{},
		&models.Upload{},
		&models.Setting{},
		&models.JobEvent{},
		&models.DramaProject{},
	); err != nil {
		return nil, err
	}
	return conn, nil
}
