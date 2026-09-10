package orchestrator

import (
	"fmt"
	"os/exec"
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

func killPort(port string) {
	port = strings.TrimSpace(port)
	if port == "" {
		return
	}
	for _, pid := range pidsOnPort(port) {
		_ = killTree(pid)
	}
}
