package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"aishow/internal/api"
	"aishow/internal/auth"
	"aishow/internal/config"
	"aishow/internal/db"
	"aishow/internal/hub"
	"aishow/internal/metrics"
	"aishow/internal/models"
	"aishow/internal/orchestrator"
	"aishow/internal/queue"
	"aishow/internal/settings"
	"aishow/internal/storage"
	"aishow/internal/worker"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	abs, _ := filepath.Abs(cfg.DataDir)
	log.Printf("aishow data dir: %s", abs)

	conn, err := db.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := settings.Seed(conn, cfg); err != nil {
		log.Fatal(err)
	}
	if err := auth.Bootstrap(conn, cfg); err != nil {
		log.Fatal(err)
	}
	if err := auth.EnsureAdmin(conn); err != nil {
		log.Fatal(err)
	}
	store, err := storage.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	h := hub.New()
	q := queue.New(conn, h)
	hw := metrics.Start()
	orch := orchestrator.New(cfg)
	orch.Start(func() models.SettingsPayload {
		return settings.Snapshot(conn, cfg)
	})
	w := worker.New(cfg, conn, q, store, orch)
	w.Start()

	srv := api.New(cfg, conn, q, store, h, hw, orch)
	addr := cfg.ListenAddr()
	log.Printf("aishow control plane listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}
