package orchestrator

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func processGone(pid int) bool {
	if pid <= 0 {
		return true
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
		if err != nil {
			return false
		}
		return !strings.Contains(strings.ToLower(string(out)), strconv.Itoa(pid))
	}
	return exec.Command("kill", "-0", strconv.Itoa(pid)).Run() != nil
}

func killTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	}
	return exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
}

func sidecarLogPath(root, engine string) string {
	name := strings.TrimSpace(engine)
	if name == "" {
		name = "sidecar"
	}
	return filepath.Join(root, "backend", "data", "sidecar-logs", name+".log")
}

func tailSidecarLog(root, engine string, n int) string {
	return tailFile(sidecarLogPath(root, engine), n)
}

func tailFile(path string, n int) string {
	if n < 1 {
		n = 12
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return strings.Join(kept, " | ")
}

func healthFatal(body map[string]any, ok bool) string {
	if !ok || body == nil {
		return ""
	}
	err, _ := body["error"].(string)
	err = strings.TrimSpace(err)
	if err == "" {
		return ""
	}
	if ready, _ := body["ready"].(bool); ready {
		return ""
	}
	if loading, _ := body["loading"].(bool); loading {
		return ""
	}
	return clipString(err, 280)
}

func clipString(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if n < 1 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func gpuMemoryMB() (used, total int, ok bool) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.used,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return 0, 0, false
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	if !sc.Scan() {
		return 0, 0, false
	}
	parts := strings.Split(sc.Text(), ",")
	if len(parts) < 2 {
		return 0, 0, false
	}
	used, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	total, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || total <= 0 {
		return 0, 0, false
	}
	return used, total, true
}

func killPort(port string) {
	port = strings.TrimSpace(port)
	if port == "" {
		return
	}
	for _, pid := range pidsOnPort(port) {
		_ = killTree(pid)
	}
}
