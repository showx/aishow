package drama

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ExtractLastFrame(videoPath, outPath string) error {
	videoPath = strings.TrimSpace(videoPath)
	if videoPath == "" {
		return fmt.Errorf("缺少视频路径")
	}
	if _, err := os.Stat(videoPath); err != nil {
		return fmt.Errorf("视频不存在，无法抽尾帧")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("未找到 ffmpeg，无法抽取尾帧")
	}
	if err := os.MkdirAll(parentDir(outPath), 0o755); err != nil {
		return err
	}
	attempts := [][]string{
		{"-y", "-hide_banner", "-loglevel", "error", "-sseof", "-0.08", "-i", videoPath, "-frames:v", "1", "-q:v", "2", outPath},
		{"-y", "-hide_banner", "-loglevel", "error", "-sseof", "-1", "-i", videoPath, "-update", "1", "-q:v", "2", outPath},
	}
	var lastErr error
	for _, args := range attempts {
		_ = os.Remove(outPath)
		cmd := exec.Command("ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
			continue
		}
		st, err := os.Stat(outPath)
		if err == nil && st.Size() > 0 {
			return nil
		}
		lastErr = fmt.Errorf("ffmpeg 没有写出尾帧")
	}
	return fmt.Errorf("抽取尾帧失败: %w", lastErr)
}

func parentDir(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i <= 0 {
		return "."
	}
	return path[:i]
}
