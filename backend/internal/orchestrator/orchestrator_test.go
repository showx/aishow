package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthFatal(t *testing.T) {
	if got := healthFatal(nil, false); got != "" {
		t.Fatalf("offline: %q", got)
	}
	if got := healthFatal(map[string]any{"error": "oom", "loading": true}, true); got != "" {
		t.Fatalf("loading should wait: %q", got)
	}
	if got := healthFatal(map[string]any{"error": "oom", "ready": true}, true); got != "" {
		t.Fatalf("ready: %q", got)
	}
	got := healthFatal(map[string]any{"error": " CUDA out of memory ", "loading": false, "ready": false}, true)
	if !strings.Contains(got, "CUDA out of memory") {
		t.Fatalf("got %q", got)
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "llada-image.log")
	if tailFile(p, 8) != "" {
		t.Fatal("missing file should be empty")
	}
	body := "a\n\nb\nc\nd\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := tailFile(p, 2)
	if got != "c | d" {
		t.Fatalf("got %q", got)
	}
}

func TestSidecarLogPath(t *testing.T) {
	got := sidecarLogPath(`D:\code\aishow`, "llada-image")
	if !strings.HasSuffix(got, filepath.Join("backend", "data", "sidecar-logs", "llada-image.log")) {
		t.Fatalf("got %q", got)
	}
}

func TestClipString(t *testing.T) {
	if got := clipString("  hello\nworld  ", 20); got != "hello world" {
		t.Fatalf("got %q", got)
	}
	if got := clipString("一二三四五", 3); got != "一二三…" {
		t.Fatalf("got %q", got)
	}
}
