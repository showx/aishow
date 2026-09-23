package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Host               string
	Port               string
	DBPath             string
	DataDir            string
	PublicBaseURL      string
	InferenceMode      string
	SGLANGFL2VAURL     string
	SGLANGRef2VAURL    string
	MediaFilePrefix    string
	URIMode            string
	WorkerConcurrency  int
	MiniMaxAPIBase     string
	MiniMaxAPIToken    string
	FastH3URL          string
	H3TurboURL         string
	H3PinkCherryURL    string
	PinkCherryComfyURL string
	H3DirectorURL      string
	LLaDAImageURL      string
	QwenImageURL       string
	HunyuanVideoURL    string
	LTX23URL           string
	ChatURL            string
	ChatModel          string
	ChatAPIToken       string
	TTSURL             string
	TTSModel           string
	TTSVoice           string
	TTSAPIToken        string
	AutoSwitchEngine   bool
	MaxLoadedEngines   int
	RepoRoot           string
	ComfyURL           string
	AdminUser          string
	AdminPassword      string
	AllowRegister      bool
	SessionDays        int
	CORSOrigins        []string
	CORSLan            bool
}

func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		Host:               env("AISHOW_HOST", "0.0.0.0"),
		Port:               env("AISHOW_PORT", "9808"),
		DBPath:             env("AISHOW_DB_PATH", "./data/aishow.db"),
		DataDir:            env("AISHOW_DATA_DIR", "./data"),
		PublicBaseURL:      strings.TrimRight(env("AISHOW_PUBLIC_BASE_URL", "http://127.0.0.1:9808"), "/"),
		InferenceMode:      strings.ToLower(env("AISHOW_INFERENCE_MODE", "mock")),
		SGLANGFL2VAURL:     strings.TrimRight(env("AISHOW_SGLANG_FL2VA_URL", "http://127.0.0.1:30010"), "/"),
		SGLANGRef2VAURL:    strings.TrimRight(env("AISHOW_SGLANG_REF2VA_URL", "http://127.0.0.1:30011"), "/"),
		MediaFilePrefix:    strings.TrimRight(env("AISHOW_MEDIA_FILE_PREFIX", "file:///data/minimax-h3"), "/"),
		URIMode:            strings.ToLower(env("AISHOW_URI_MODE", "file")),
		WorkerConcurrency:  envInt("AISHOW_WORKER_CONCURRENCY", 1),
		MiniMaxAPIBase:     strings.TrimRight(env("AISHOW_MINIMAX_API_BASE", "https://api.minimaxi.com"), "/"),
		MiniMaxAPIToken:    env("AISHOW_MINIMAX_API_TOKEN", ""),
		FastH3URL:          strings.TrimRight(env("AISHOW_FASTH3_URL", "http://127.0.0.1:8000"), "/"),
		H3TurboURL:         strings.TrimRight(env("AISHOW_H3_TURBO_URL", "http://127.0.0.1:30012"), "/"),
		H3PinkCherryURL:    strings.TrimRight(env("AISHOW_H3_PINKCHERRY_URL", "http://127.0.0.1:30013"), "/"),
		PinkCherryComfyURL: strings.TrimRight(env("AISHOW_PINKCHERRY_COMFY_URL", "http://127.0.0.1:8189"), "/"),
		H3DirectorURL:      strings.TrimRight(env("AISHOW_H3_DIRECTOR_URL", "http://127.0.0.1:30014"), "/"),
		LLaDAImageURL:      strings.TrimRight(env("AISHOW_LLADA_IMAGE_URL", "http://127.0.0.1:30020"), "/"),
		QwenImageURL:       strings.TrimRight(env("AISHOW_QWEN_IMAGE_URL", "http://127.0.0.1:30021"), "/"),
		HunyuanVideoURL:    strings.TrimRight(env("AISHOW_HUNYUAN_VIDEO_URL", "http://127.0.0.1:30022"), "/"),
		LTX23URL:           strings.TrimRight(env("AISHOW_LTX23_URL", "http://127.0.0.1:30023"), "/"),
		ChatURL:            strings.TrimRight(env("AISHOW_CHAT_URL", "http://127.0.0.1:11434"), "/"),
		ChatModel:          env("AISHOW_CHAT_MODEL", "qwen3.5:4b"),
		ChatAPIToken:       env("AISHOW_CHAT_API_TOKEN", ""),
		TTSURL:             strings.TrimRight(env("AISHOW_TTS_URL", ""), "/"),
		TTSModel:           env("AISHOW_TTS_MODEL", ""),
		TTSVoice:           env("AISHOW_TTS_VOICE", "alloy"),
		TTSAPIToken:        env("AISHOW_TTS_API_TOKEN", ""),
		AutoSwitchEngine:   envBool("AISHOW_AUTO_SWITCH_ENGINE", true),
		MaxLoadedEngines:   envInt("AISHOW_MAX_LOADED_ENGINES", 1),
		RepoRoot:           env("AISHOW_ROOT", ""),
		ComfyURL:           strings.TrimRight(env("AISHOW_COMFY_URL", "http://127.0.0.1:8188"), "/"),
		AdminUser:          env("AISHOW_ADMIN_USER", ""),
		AdminPassword:      env("AISHOW_ADMIN_PASSWORD", ""),
		AllowRegister:      envBool("AISHOW_ALLOW_REGISTER", true),
		SessionDays:        envInt("AISHOW_SESSION_DAYS", 30),
		CORSOrigins: uniqueStrings(append(
			[]string{"http://127.0.0.1:5173", "http://localhost:5173"},
			envList("AISHOW_CORS_ORIGINS")...,
		)),
		CORSLan: envBool("AISHOW_CORS_LAN", true),
	}
	if cfg.WorkerConcurrency < 1 {
		cfg.WorkerConcurrency = 1
	}
	if cfg.MaxLoadedEngines < 1 {
		cfg.MaxLoadedEngines = 1
	}
	if cfg.SessionDays < 1 {
		cfg.SessionDays = 30
	}
	return cfg
}

func (c Config) ListenAddr() string {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	return host + ":" + c.Port
}

func (c Config) AllowCORSOrigin(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return false
	}
	for _, o := range c.CORSOrigins {
		if origin == strings.TrimRight(o, "/") {
			return true
		}
	}
	if c.CORSLan {
		return isPrivateHTTPOrigin(origin)
	}
	return false
}

func isPrivateHTTPOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback()
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envList(key string) []string {
	var out []string
	for _, p := range strings.Split(os.Getenv(key), ",") {
		p = strings.TrimRight(strings.TrimSpace(p), "/")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimRight(strings.TrimSpace(s), "/")
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
