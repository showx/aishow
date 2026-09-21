package drama

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ConcatOptions struct {
	Width, Height int
	Cues          []SubtitleCue
	ClipAudio     []string
}

func ConcatVideos(paths []string, outPath string, opts ConcatOptions) error {
	if len(paths) == 0 {
		return fmt.Errorf("没有可合并的视频")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("未找到 ffmpeg，无法合成短片。请安装 ffmpeg 并加入 PATH")
	}
	w, h := opts.Width, opts.Height
	if w <= 0 || h <= 0 {
		w, h = 480, 854
	}
	w, h = even(w), even(h)

	tmpDir, err := os.MkdirTemp("", "aishow-drama-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var list strings.Builder
	for i, inPath := range paths {
		inPath = strings.TrimSpace(inPath)
		if inPath == "" {
			return fmt.Errorf("第 %d 段视频路径为空", i+1)
		}
		if _, err := os.Stat(inPath); err != nil {
			return fmt.Errorf("第 %d 段视频不存在（模拟任务没有真实 MP4，无法拼接）", i+1)
		}
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip-%02d.mp4", i+1))
		voice := ""
		if i < len(opts.ClipAudio) {
			voice = strings.TrimSpace(opts.ClipAudio[i])
		}
		if err := transcodeClip(inPath, clipPath, w, h, voice); err != nil {
			return fmt.Errorf("处理第 %d 段失败: %w", i+1, err)
		}
		fmt.Fprintf(&list, "file %s\n", concatListEscape(clipPath))
	}

	listPath := filepath.Join(tmpDir, "list.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}
	joined := filepath.Join(tmpDir, "joined.mp4")
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy", "-movflags", "+faststart",
		joined,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("拼接失败: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	final := joined
	if len(opts.Cues) > 0 {
		assPath := filepath.Join(tmpDir, "subs.ass")
		if err := os.WriteFile(assPath, []byte(WriteASS(opts.Cues, w, h)), 0o644); err != nil {
			return err
		}
		burned := filepath.Join(tmpDir, "burned.mp4")
		vf := "ass=" + ffmpegFilterPath(assPath)
		burn := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-i", joined, "-vf", vf,
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p",
			"-c:a", "copy", "-movflags", "+faststart",
			burned,
		}
		bctx, bcancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer bcancel()
		bcmd := exec.CommandContext(bctx, "ffmpeg", burn...)
		var berr bytes.Buffer
		bcmd.Stderr = &berr
		if err := bcmd.Run(); err != nil {
			return fmt.Errorf("烧录字幕失败: %v: %s", err, strings.TrimSpace(berr.String()))
		}
		final = burned
	}
	st, err := os.Stat(final)
	if err != nil || st.Size() == 0 {
		return fmt.Errorf("拼接没有写出文件")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(outPath)
	if err := os.Rename(final, outPath); err != nil {
		in, err := os.Open(final)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := out.ReadFrom(in); err != nil {
			return err
		}
	}
	return nil
}

func transcodeClip(inPath, outPath string, w, h int, voicePath string) error {
	silent := !ffmpegHasAudio(inPath)
	hasVoice := strings.TrimSpace(voicePath) != ""
	if hasVoice {
		if _, err := os.Stat(voicePath); err != nil {
			hasVoice = false
		}
	}
	_ = os.Remove(outPath)
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=24,format=yuv420p", w, h, w, h)
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", inPath}
	if silent && !hasVoice {
		args = append(args, "-f", "lavfi", "-t", "3600", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100")
	}
	if hasVoice {
		args = append(args, "-i", voicePath)
	}
	args = append(args, "-vf", vf, "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p")
	switch {
	case hasVoice && silent:
		args = append(args, "-map", "0:v:0", "-map", "1:a:0", "-c:a", "aac", "-ar", "44100", "-ac", "2")
	case hasVoice:
		args = append(args,
			"-filter_complex", "[0:a]volume=0.35[orig];[1:a]volume=1.15,aformat=sample_rates=44100:channel_layouts=stereo[voice];[orig][voice]amix=inputs=2:duration=first:dropout_transition=0[a]",
			"-map", "0:v:0", "-map", "[a]", "-c:a", "aac", "-ar", "44100", "-ac", "2",
		)
	default:
		args = append(args, "-c:a", "aac", "-ar", "44100", "-ac", "2")
	}
	args = append(args, "-shortest", "-movflags", "+faststart", outPath)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("转码失败: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	st, err := os.Stat(outPath)
	if err != nil || st.Size() == 0 {
		return fmt.Errorf("转码没有写出文件")
	}
	return nil
}

func ffmpegHasAudio(path string) bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-i", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()
	return strings.Contains(stderr.String(), "Audio:")
}

func concatListEscape(path string) string {
	p := filepath.ToSlash(path)
	p = strings.ReplaceAll(p, "'", `'\''`)
	return "'" + p + "'"
}
