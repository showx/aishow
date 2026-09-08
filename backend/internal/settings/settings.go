package settings

import (
	"strconv"
	"strings"

	"aishow/internal/config"
	"aishow/internal/models"

	"gorm.io/gorm"
)

const (
	KeyInferenceMode     = "inference_mode"
	KeyFL2VA             = "sglang_fl2va_url"
	KeyRef2VA            = "sglang_ref2va_url"
	KeyMediaPrefix       = "media_file_prefix"
	KeyURIMode           = "uri_mode"
	KeyWorkerConcurrency = "worker_concurrency"
	KeyPublicBase        = "public_base_url"
	KeyMiniMaxBase       = "minimax_api_base"
	KeyMiniMaxToken      = "minimax_api_token"
	KeyFastH3            = "fasth3_url"
	KeyLLaDAImage        = "llada_image_url"
	KeyAllowRegister     = "allow_register"
)

func Seed(db *gorm.DB, cfg config.Config) error {
	defaults := map[string]string{
		KeyInferenceMode:     cfg.InferenceMode,
		KeyFL2VA:             cfg.SGLANGFL2VAURL,
		KeyRef2VA:            cfg.SGLANGRef2VAURL,
		KeyMediaPrefix:       cfg.MediaFilePrefix,
		KeyURIMode:           cfg.URIMode,
		KeyWorkerConcurrency: strconv.Itoa(cfg.WorkerConcurrency),
		KeyPublicBase:        cfg.PublicBaseURL,
		KeyMiniMaxBase:       cfg.MiniMaxAPIBase,
		KeyMiniMaxToken:      cfg.MiniMaxAPIToken,
		KeyFastH3:            cfg.FastH3URL,
		KeyLLaDAImage:        cfg.LLaDAImageURL,
		KeyAllowRegister:     boolString(cfg.AllowRegister),
	}
	for k, v := range defaults {
		row := models.Setting{Key: k, Value: v}
		if err := db.Where("key = ?", k).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func Get(db *gorm.DB, key, fallback string) string {
	var row models.Setting
	if err := db.First(&row, "key = ?", key).Error; err != nil {
		return fallback
	}
	if strings.TrimSpace(row.Value) == "" {
		return fallback
	}
	return row.Value
}

func Put(db *gorm.DB, key, value string) error {
	return db.Save(&models.Setting{Key: key, Value: value}).Error
}

func Snapshot(db *gorm.DB, cfg config.Config) models.SettingsPayload {
	token := Get(db, KeyMiniMaxToken, cfg.MiniMaxAPIToken)
	masked := ""
	if token != "" {
		masked = "********"
	}
	return models.SettingsPayload{
		InferenceMode:     Get(db, KeyInferenceMode, cfg.InferenceMode),
		SGLANGFL2VAURL:    Get(db, KeyFL2VA, cfg.SGLANGFL2VAURL),
		SGLANGRef2VAURL:   Get(db, KeyRef2VA, cfg.SGLANGRef2VAURL),
		MediaFilePrefix:   Get(db, KeyMediaPrefix, cfg.MediaFilePrefix),
		URIMode:           Get(db, KeyURIMode, cfg.URIMode),
		WorkerConcurrency: atoi(Get(db, KeyWorkerConcurrency, strconv.Itoa(cfg.WorkerConcurrency))),
		PublicBaseURL:     Get(db, KeyPublicBase, cfg.PublicBaseURL),
		MiniMaxAPIBase:    Get(db, KeyMiniMaxBase, cfg.MiniMaxAPIBase),
		MiniMaxAPIToken:   masked,
		HasMiniMaxToken:   token != "",
		FastH3URL:         Get(db, KeyFastH3, cfg.FastH3URL),
		LLaDAImageURL:     Get(db, KeyLLaDAImage, cfg.LLaDAImageURL),
	}
}

func Apply(db *gorm.DB, in models.SettingsPayload) error {
	pairs := map[string]string{
		KeyInferenceMode:     strings.ToLower(strings.TrimSpace(in.InferenceMode)),
		KeyFL2VA:             strings.TrimRight(strings.TrimSpace(in.SGLANGFL2VAURL), "/"),
		KeyRef2VA:            strings.TrimRight(strings.TrimSpace(in.SGLANGRef2VAURL), "/"),
		KeyMediaPrefix:       strings.TrimRight(strings.TrimSpace(in.MediaFilePrefix), "/"),
		KeyURIMode:           strings.ToLower(strings.TrimSpace(in.URIMode)),
		KeyWorkerConcurrency: strconv.Itoa(in.WorkerConcurrency),
		KeyPublicBase:        strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/"),
		KeyMiniMaxBase:       strings.TrimRight(strings.TrimSpace(in.MiniMaxAPIBase), "/"),
		KeyFastH3:            strings.TrimRight(strings.TrimSpace(in.FastH3URL), "/"),
		KeyLLaDAImage:        strings.TrimRight(strings.TrimSpace(in.LLaDAImageURL), "/"),
	}
	for k, v := range pairs {
		if v == "" && k != KeyMiniMaxBase {
			continue
		}
		if err := Put(db, k, v); err != nil {
			return err
		}
	}
	if in.MiniMaxAPIToken != "" && in.MiniMaxAPIToken != "********" {
		if err := Put(db, KeyMiniMaxToken, strings.TrimSpace(in.MiniMaxAPIToken)); err != nil {
			return err
		}
	}
	return nil
}

func AllowRegister(db *gorm.DB, cfg config.Config) bool {
	v := strings.ToLower(strings.TrimSpace(Get(db, KeyAllowRegister, "")))
	if v == "" {
		return cfg.AllowRegister
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func SetAllowRegister(db *gorm.DB, on bool) error {
	return Put(db, KeyAllowRegister, boolString(on))
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	if n < 1 {
		return 1
	}
	return n
}
