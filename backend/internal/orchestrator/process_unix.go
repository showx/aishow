//go:build !windows

package orchestrator

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func startScript(script, workDir string) (*os.Process, error) {
	cmd := exec.Command("bash", script)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "AISHOW_HEADLESS=1")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go cmd.Wait()
	return cmd.Process, nil
}

func pidsOnPort(port string) []int {
	out, err := exec.Command("sh", "-c", "lsof -t -iTCP:"+port+" -sTCP:LISTEN 2>/dev/null || fuser "+port+"/tcp 2>/dev/null").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, part := range strings.Fields(string(out)) {
		n, err := strconv.Atoi(part)
		if err == nil && n > 0 {
			pids = append(pids, n)
		}
	}
	return pids
}
