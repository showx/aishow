//go:build windows

package orchestrator

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func startScript(script, workDir string) (*os.Process, error) {
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "AISHOW_HEADLESS=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // CREATE_NEW_CONSOLE
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go cmd.Wait()
	return cmd.Process, nil
}

func pidsOnPort(port string) []int {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil
	}
	suffix := ":" + port
	seen := map[int]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.ToUpper(strings.TrimSpace(sc.Text()))
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !strings.HasSuffix(fields[1], suffix) {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}
		seen[pid] = struct{}{}
	}
	outPids := make([]int, 0, len(seen))
	for pid := range seen {
		outPids = append(outPids, pid)
	}
	return outPids
}
