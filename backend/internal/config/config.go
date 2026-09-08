package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DBPath            string
	DataDir           string
	PublicBaseURL     string
	InferenceMode     string
	SGLANGFL2VAURL    string
	SGLANGRef2VAURL   string
	MediaFilePrefix   string
	URIMode           string
	WorkerConcurrency int
	MiniMaxAPIBase    string
	MiniMaxAPIToken   string
	FastH3URL         string
	LLaDAImageURL     string
}

func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		Port:              env("AISHOW_PORT", "9808"),
		DBPath:            env("AISHOW_DB_PATH", "./data/aishow.db"),
		DataDir:           env("AISHOW_DATA_DIR", "./data"),
		PublicBaseURL:     strings.TrimRight(env("AISHOW_PUBLIC_BASE_URL", "http://127.0.0.1:9808"), "/"),
		InferenceMode:     strings.ToLower(env("AISHOW_INFERENCE_MODE", "mock")),
		SGLANGFL2VAURL:    strings.TrimRight(env("AISHOW_SGLANG_FL2VA_URL", "http://127.0.0.1:30010"), "/"),
		SGLANGRef2VAURL:   strings.TrimRight(env("AISHOW_SGLANG_REF2VA_URL", "http://127.0.0.1:30011"), "/"),
		MediaFilePrefix:   strings.TrimRight(env("AISHOW_MEDIA_FILE_PREFIX", "file:///data/minimax-h3"), "/"),
		URIMode:           strings.ToLower(env("AISHOW_URI_MODE", "file")),
		WorkerConcurrency: envInt("AISHOW_WORKER_CONCURRENCY", 1),
		MiniMaxAPIBase:    strings.TrimRight(env("AISHOW_MINIMAX_API_BASE", "https://api.minimaxi.com"), "/"),
		MiniMaxAPIToken:   env("AISHOW_MINIMAX_API_TOKEN", ""),
		FastH3URL:         strings.TrimRight(env("AISHOW_FASTH3_URL", "http://127.0.0.1:8000"), "/"),
		LLaDAImageURL:     strings.TrimRight(env("AISHOW_LLADA_IMAGE_URL", "http://127.0.0.1:30020"), "/"),
	}
	if cfg.WorkerConcurrency < 1 {
		cfg.WorkerConcurrency = 1
	}
	return cfg
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
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
