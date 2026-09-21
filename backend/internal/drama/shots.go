package drama

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"aishow/internal/models"
)

const ContinueCue = "将@视频1从最后一帧向后延长，承接结尾的姿态、走位与运镜，不要重新开场。"

func ParseShots(raw string) []models.DramaShot {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var shots []models.DramaShot
	if err := json.Unmarshal([]byte(raw), &shots); err != nil || !isShotArray(shots, raw) {
		var wrap struct {
			Shots []models.DramaShot `json:"shots"`
		}
		if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
			return nil
		}
		shots = wrap.Shots
	}
	return NormalizeShots(shots)
}

func isShotArray(shots []models.DramaShot, raw string) bool {
	trim := strings.TrimSpace(raw)
	return strings.HasPrefix(trim, "[") || len(shots) > 0
}

func NormalizeShots(shots []models.DramaShot) []models.DramaShot {
	out := make([]models.DramaShot, 0, len(shots))
	for _, s := range shots {
		if len(out) >= models.DramaMaxShots {
			break
		}
		s.Title = strings.TrimSpace(s.Title)
		s.Scene = strings.TrimSpace(s.Scene)
		s.Dialogue = strings.TrimSpace(s.Dialogue)
		s.ImagePrompt = strings.TrimSpace(s.ImagePrompt)
		s.VideoPrompt = strings.TrimSpace(s.VideoPrompt)
		if s.Duration <= 0 {
			s.Duration = 5
		}
		if s.Duration < 2 {
			s.Duration = 2
		}
		if s.Duration > 30 {
			s.Duration = 30
		}
		s.Beats = normalizeBeats(s.Beats)
		s.ImageRefs = CleanImageRefs(s.ImageRefs)
		s.LastFrameUploadID = strings.TrimSpace(s.LastFrameUploadID)
		s.Index = len(out) + 1
		if s.Title == "" {
			s.Title = fmt.Sprintf("第 %d 镜", s.Index)
		}
		out = append(out, s)
	}
	return out
}

func normalizeBeats(beats []models.DramaShotBeat) []models.DramaShotBeat {
	out := make([]models.DramaShotBeat, 0, len(beats))
	for _, b := range beats {
		b.TimeRange = strings.TrimSpace(b.TimeRange)
		b.Action = strings.TrimSpace(b.Action)
		b.ImagePrompt = strings.TrimSpace(b.ImagePrompt)
		if b.TimeRange == "" && b.Action == "" && b.ImagePrompt == "" {
			continue
		}
		if len(out) >= models.DramaMaxBeats {
			break
		}
		b.Index = len(out) + 1
		out = append(out, b)
	}
	return out
}

func EncodeShots(shots []models.DramaShot) string {
	shots = NormalizeShots(shots)
	if shots == nil {
		shots = []models.DramaShot{}
	}
	b, err := json.Marshal(shots)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func EmptyShot(index int) models.DramaShot {
	if index < 1 {
		index = 1
	}
	return models.DramaShot{
		Index:            index,
		Title:            fmt.Sprintf("第 %d 镜", index),
		Duration:         5,
		ContinueFromPrev: index > 1,
	}
}

func SuggestedShotCount(targetSec int) int {
	if targetSec <= 0 {
		targetSec = 60
	}
	n := (targetSec + 4) / 5
	if n < 1 {
		n = 1
	}
	if n > models.DramaMaxShots {
		n = models.DramaMaxShots
	}
	return n
}

func FillEmptyShots(n int) []models.DramaShot {
	if n < 1 {
		n = 1
	}
	if n > models.DramaMaxShots {
		n = models.DramaMaxShots
	}
	out := make([]models.DramaShot, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, EmptyShot(i))
	}
	return out
}

func ShotStoryboardReady(s models.DramaShot) bool {
	return strings.TrimSpace(s.Scene) != "" || strings.TrimSpace(s.ImagePrompt) != ""
}

func AllStoryboardReady(shots []models.DramaShot) bool {
	if len(shots) == 0 {
		return false
	}
	for _, s := range shots {
		if !ShotStoryboardReady(s) {
			return false
		}
	}
	return true
}

func JobBusy(status string) bool {
	switch status {
	case models.StatusQueued, models.StatusRunning:
		return true
	default:
		return false
	}
}

func JobSucceeded(job *models.Job) bool {
	return job != nil && job.Status == models.StatusSucceeded
}

func JobFailed(job *models.Job) bool {
	return job != nil && (job.Status == models.StatusFailed || job.Status == models.StatusCancelled)
}

func EnsureContinuePrompt(prompt string) string {
	p := strings.TrimSpace(prompt)
	p = strings.ReplaceAll(p, "参考视频1", "@视频1")
	p = strings.ReplaceAll(p, "参考 @视频1", "@视频1")
	if strings.Contains(p, "从最后一帧向后延长") {
		if !strings.Contains(p, "@视频1") {
			p = "@视频1 " + p
		}
		return strings.TrimSpace(p)
	}
	if p == "" {
		return ContinueCue
	}
	return ContinueCue + "\n" + p
}

func ImagePrompt(p *models.DramaProject, base string, withPrev bool) string {
	text := strings.TrimSpace(base)
	var b strings.Builder
	if text != "" {
		b.WriteString(text)
	}
	if p != nil && strings.TrimSpace(p.Style) != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("风格：")
		b.WriteString(strings.TrimSpace(p.Style))
	}
	if p != nil && strings.TrimSpace(p.StyleNotes) != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("风格注意事项：")
		b.WriteString(strings.TrimSpace(p.StyleNotes))
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("出图提示与画面说明一律使用中文。一张图只表现一个机位、一个瞬间。")
	if withPrev {
		b.WriteString("人物外形必须与参考图保持一致。")
	}
	return strings.TrimSpace(b.String())
}

func VideoPrompt(shot models.DramaShot, styleNotes string) string {
	p := firstNonEmpty(shot.VideoPrompt, shot.Scene, shot.ImagePrompt)
	if strings.TrimSpace(shot.Dialogue) != "" {
		p = strings.TrimSpace(p + "\n对白：" + strings.TrimSpace(shot.Dialogue))
	}
	if len(shot.Beats) > 0 {
		p += "\n按节拍依次表演，时间轴必须对齐："
		for _, beat := range shot.Beats {
			line := strings.TrimSpace(strings.TrimSpace(beat.TimeRange) + " " + strings.TrimSpace(beat.Action))
			if line != "" {
				p += "\n" + line
			}
		}
	}
	if shot.ContinueFromPrev {
		p = EnsureContinuePrompt(p)
	}
	if strings.TrimSpace(styleNotes) != "" {
		p = strings.TrimSpace(p + "\n风格注意事项：" + strings.TrimSpace(styleNotes))
	}
	return strings.TrimSpace(p)
}

func RequireScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("请先填写剧本")
	}
	return nil
}

func RequireStoryboard(shots []models.DramaShot) error {
	if len(shots) == 0 {
		return fmt.Errorf("请先拆分镜")
	}
	var pending []int
	for _, s := range shots {
		if !ShotStoryboardReady(s) {
			pending = append(pending, s.Index)
		}
	}
	if len(pending) > 0 {
		return fmt.Errorf("还有 %d 镜分镜不完整，补全画面或出图提示后再出图", len(pending))
	}
	return nil
}

func RequireImages(shots []models.DramaShotView) error {
	if len(shots) == 0 {
		return fmt.Errorf("请先拆分镜并出图")
	}
	var pending []int
	for _, s := range shots {
		if !JobSucceeded(s.ImageJob) {
			pending = append(pending, s.Index)
		}
	}
	if len(pending) > 0 {
		return fmt.Errorf("还有 %d 镜未出完图，全部完成后再进入成片", len(pending))
	}
	return nil
}

func RequireSomeVideos(shots []models.DramaShotView) error {
	for _, s := range shots {
		if JobSucceeded(s.VideoJob) {
			return nil
		}
	}
	return fmt.Errorf("请先完成至少一镜成片，再筛选合并")
}

func RequireStep(step, script string, shots []models.DramaShot, views []models.DramaShotView) error {
	switch step {
	case models.DramaStepWrite:
		return nil
	case models.DramaStepStoryboard:
		return RequireScript(script)
	case models.DramaStepImage:
		if err := RequireScript(script); err != nil {
			return err
		}
		return RequireStoryboard(shots)
	case models.DramaStepVideo:
		if err := RequireScript(script); err != nil {
			return err
		}
		if err := RequireStoryboard(shots); err != nil {
			return err
		}
		return RequireImages(views)
	case models.DramaStepCompile:
		if err := RequireScript(script); err != nil {
			return err
		}
		if err := RequireStoryboard(shots); err != nil {
			return err
		}
		return RequireSomeVideos(views)
	default:
		return fmt.Errorf("未知步骤")
	}
}

func ParseIndexes(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var out []int
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func EncodeIndexes(indexes []int) string {
	if indexes == nil {
		indexes = []int{}
	}
	b, err := json.Marshal(indexes)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func ClipRunes(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func CompileSize(aspect string, shortEdge int) (w, h int) {
	if shortEdge <= 0 {
		shortEdge = 480
	}
	shortEdge = even(shortEdge)
	switch strings.TrimSpace(aspect) {
	case "16:9":
		return even(shortEdge * 16 / 9), shortEdge
	case "4:3":
		return even(shortEdge * 4 / 3), shortEdge
	case "3:4":
		return shortEdge, even(shortEdge * 4 / 3)
	case "1:1":
		return shortEdge, shortEdge
	case "21:9":
		return even(shortEdge * 21 / 9), shortEdge
	default:
		return shortEdge, even(shortEdge * 16 / 9)
	}
}

func even(n int) int {
	if n < 2 {
		return 2
	}
	if n%2 != 0 {
		n++
	}
	return n
}
