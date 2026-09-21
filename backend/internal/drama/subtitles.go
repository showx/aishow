package drama

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"aishow/internal/models"
)

type SubtitleCue struct {
	Start float64
	End   float64
	Text  string
}

func BuildSubtitleCues(shots []models.DramaShotView, indexes []int) []SubtitleCue {
	want := map[int]bool{}
	for _, idx := range indexes {
		want[idx] = true
	}
	var cues []SubtitleCue
	var t float64
	for _, sh := range shots {
		if len(want) > 0 && !want[sh.Index] {
			continue
		}
		dur := sh.Duration
		if dur <= 0 {
			dur = 5
		}
		text := strings.TrimSpace(sh.Dialogue)
		if text != "" {
			cues = append(cues, SubtitleCue{Start: t, End: t + dur, Text: text})
		}
		t += dur
	}
	return cues
}

func WriteASS(cues []SubtitleCue, w, h int) string {
	if w <= 0 {
		w = 480
	}
	if h <= 0 {
		h = 854
	}
	font := 36
	if h >= 1000 {
		font = 48
	}
	var b strings.Builder
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	fmt.Fprintf(&b, "PlayResX: %d\nPlayResY: %d\nWrapStyle: 0\nScaledBorderAndShadow: yes\n\n", w, h)
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Default,Microsoft YaHei,%d,&H00FFFFFF,&H000000FF,&H00101010,&H80000000,0,0,0,0,100,100,0,0,1,3,0,2,28,28,56,1\n\n", font)
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, cue := range cues {
		text := escapeASS(cue.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", assTime(cue.Start), assTime(cue.End), text)
	}
	return b.String()
}

func assTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	cs := int(sec*100 + 0.5)
	h := cs / 360000
	cs %= 360000
	m := cs / 6000
	cs %= 6000
	s := cs / 100
	cs %= 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

func escapeASS(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\\N")
	s = strings.ReplaceAll(s, "\n", "\\N")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	if utf8.RuneCountInString(s) > 80 {
		r := []rune(s)
		s = string(r[:80])
	}
	return s
}

func ffmpegFilterPath(path string) string {
	p := strings.ReplaceAll(path, `\`, `/`)
	p = strings.ReplaceAll(p, ":", `\:`)
	p = strings.ReplaceAll(p, "'", `\'`)
	return p
}
