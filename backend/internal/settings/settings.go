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
	KeyH3Turbo           = "h3_turbo_url"
	KeyH3PinkCherry      = "h3_pinkcherry_url"
	KeyH3Director        = "h3_director_url"
	KeyLLaDAImage        = "llada_image_url"
	KeyQwenImage         = "qwen_image_url"
	KeyHunyuanVideo      = "hunyuan_video_url"
	KeyLTX23             = "ltx23_url"
	KeyChatURL           = "chat_url"
	KeyChatModel         = "chat_model"
	KeyChatToken         = "chat_api_token"
	KeyTTSURL            = "tts_url"
	KeyTTSModel          = "tts_model"
	KeyTTSVoice          = "tts_voice"
	KeyTTSToken          = "tts_api_token"
	KeyAutoSwitchEngine  = "auto_switch_engine"
	KeyMaxLoadedEngines  = "max_loaded_engines"
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
		KeyH3Turbo:           cfg.H3TurboURL,
		KeyH3PinkCherry:      cfg.H3PinkCherryURL,
		KeyH3Director:        cfg.H3DirectorURL,
		KeyLLaDAImage:        cfg.LLaDAImageURL,
		KeyQwenImage:         cfg.QwenImageURL,
		KeyHunyuanVideo:      cfg.HunyuanVideoURL,
		KeyLTX23:             cfg.LTX23URL,
		KeyChatURL:           cfg.ChatURL,
		KeyChatModel:         cfg.ChatModel,
		KeyChatToken:         cfg.ChatAPIToken,
		KeyTTSURL:            cfg.TTSURL,
		KeyTTSModel:          cfg.TTSModel,
		KeyTTSVoice:          cfg.TTSVoice,
		KeyTTSToken:          cfg.TTSAPIToken,
		KeyAutoSwitchEngine:  boolString(cfg.AutoSwitchEngine),
		KeyMaxLoadedEngines:  strconv.Itoa(max1(cfg.MaxLoadedEngines)),
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
	chatToken := Get(db, KeyChatToken, cfg.ChatAPIToken)
	chatMasked := ""
	if chatToken != "" {
		chatMasked = "********"
	}
	ttsToken := Get(db, KeyTTSToken, cfg.TTSAPIToken)
	ttsMasked := ""
	if ttsToken != "" {
		ttsMasked = "********"
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
		H3TurboURL:        Get(db, KeyH3Turbo, cfg.H3TurboURL),
		H3PinkCherryURL:   Get(db, KeyH3PinkCherry, cfg.H3PinkCherryURL),
		H3DirectorURL:     Get(db, KeyH3Director, cfg.H3DirectorURL),
		LLaDAImageURL:     Get(db, KeyLLaDAImage, cfg.LLaDAImageURL),
		QwenImageURL:      Get(db, KeyQwenImage, cfg.QwenImageURL),
		HunyuanVideoURL:   Get(db, KeyHunyuanVideo, cfg.HunyuanVideoURL),
		LTX23URL:          Get(db, KeyLTX23, cfg.LTX23URL),
		ChatURL:           Get(db, KeyChatURL, cfg.ChatURL),
		ChatModel:         Get(db, KeyChatModel, cfg.ChatModel),
		ChatAPIToken:      chatMasked,
		HasChatToken:      chatToken != "",
		TTSURL:            Get(db, KeyTTSURL, cfg.TTSURL),
		TTSModel:          Get(db, KeyTTSModel, cfg.TTSModel),
		TTSVoice:          Get(db, KeyTTSVoice, cfg.TTSVoice),
		TTSAPIToken:       ttsMasked,
		HasTTSToken:       ttsToken != "",
		AutoSwitchEngine:  boolPtr(parseBool(Get(db, KeyAutoSwitchEngine, boolString(cfg.AutoSwitchEngine)), cfg.AutoSwitchEngine)),
		MaxLoadedEngines:  intPtr(max1(atoi(Get(db, KeyMaxLoadedEngines, strconv.Itoa(max1(cfg.MaxLoadedEngines)))))),
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
		KeyH3Turbo:           strings.TrimRight(strings.TrimSpace(in.H3TurboURL), "/"),
		KeyH3PinkCherry:      strings.TrimRight(strings.TrimSpace(in.H3PinkCherryURL), "/"),
		KeyH3Director:        strings.TrimRight(strings.TrimSpace(in.H3DirectorURL), "/"),
		KeyLLaDAImage:        strings.TrimRight(strings.TrimSpace(in.LLaDAImageURL), "/"),
		KeyQwenImage:         strings.TrimRight(strings.TrimSpace(in.QwenImageURL), "/"),
		KeyHunyuanVideo:      strings.TrimRight(strings.TrimSpace(in.HunyuanVideoURL), "/"),
		KeyLTX23:             strings.TrimRight(strings.TrimSpace(in.LTX23URL), "/"),
		KeyChatURL:           strings.TrimRight(strings.TrimSpace(in.ChatURL), "/"),
		KeyChatModel:         strings.TrimSpace(in.ChatModel),
		KeyTTSURL:            strings.TrimRight(strings.TrimSpace(in.TTSURL), "/"),
		KeyTTSModel:          strings.TrimSpace(in.TTSModel),
		KeyTTSVoice:          strings.TrimSpace(in.TTSVoice),
	}
	for k, v := range pairs {
		if v == "" && k != KeyMiniMaxBase && k != KeyChatModel && k != KeyChatURL && k != KeyTTSURL && k != KeyTTSModel && k != KeyTTSVoice {
			continue
		}
		if err := Put(db, k, v); err != nil {
			return err
		}
	}
	if in.MaxLoadedEngines != nil {
		if err := Put(db, KeyMaxLoadedEngines, strconv.Itoa(max1(*in.MaxLoadedEngines))); err != nil {
			return err
		}
	}
	if in.AutoSwitchEngine != nil {
		if err := Put(db, KeyAutoSwitchEngine, boolString(*in.AutoSwitchEngine)); err != nil {
			return err
		}
	}
	if in.MiniMaxAPIToken != "" && in.MiniMaxAPIToken != "********" {
		if err := Put(db, KeyMiniMaxToken, strings.TrimSpace(in.MiniMaxAPIToken)); err != nil {
			return err
		}
	}
	if in.ChatAPIToken != "" && in.ChatAPIToken != "********" {
		if err := Put(db, KeyChatToken, strings.TrimSpace(in.ChatAPIToken)); err != nil {
			return err
		}
	}
	if in.TTSAPIToken != "" && in.TTSAPIToken != "********" {
		if err := Put(db, KeyTTSToken, strings.TrimSpace(in.TTSAPIToken)); err != nil {
			return err
		}
	}
	return nil
}

func ChatToken(db *gorm.DB, cfg config.Config) string {
	return Get(db, KeyChatToken, cfg.ChatAPIToken)
}

func TTSToken(db *gorm.DB, cfg config.Config) string {
	return Get(db, KeyTTSToken, cfg.TTSAPIToken)
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

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func parseBool(s string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	if n < 1 {
		return 1
	}
	return n
}
