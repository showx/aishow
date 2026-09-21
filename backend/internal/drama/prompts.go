package drama

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"aishow/internal/models"
)

var jsonFence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
var thinkBlock = regexp.MustCompile("(?s)<think>.*?</think>")

func WriteSystemPrompt() string {
	return "你是短剧编剧。请根据用户给出的题材、风格和目标时长，写出适合竖屏短剧的完整剧本。要求：分场清晰，对白口语化，每场标明场景与情绪，控制在目标时长内。只输出剧本正文，不要解释。"
}

func WriteUserPrompt(idea, style string, targetSec int) string {
	return fmt.Sprintf("题材/意图：%s\n风格：%s\n目标时长：约 %d 秒", strings.TrimSpace(idea), emptyAs(style, "未指定"), targetSec)
}

func StoryboardSystemPrompt(count, targetSec int) string {
	count, targetSec = storyboardBudget(count, targetSec)
	return fmt.Sprintf(`你是短剧分镜师。把剧本拆成 %d 个镜头的竖屏分镜（目标约 %d 秒，最多 %d 镜）。必须返回 JSON 对象，shots 数组必须有 %d 条，禁止只返回第 1 镜。格式：
{"shots":[{"index":1,"title":"开场","scene":"画面描述（中文）","dialogue":"对白","image_prompt":"中文出图提示，只写这一个瞬间","video_prompt":"运镜与表演","duration":5,"beats":[]},{"index":2,"title":"下一镜","scene":"接着发生的画面","dialogue":"","image_prompt":"中文出图提示","video_prompt":"将@视频1从最后一帧向后延长，……","duration":5,"beats":[]}]}
规则：
- duration 为该镜建议秒数（2-15），各镜之和接近目标时长。
- image_prompt 必须全程中文：写构图、人物外形、服装、场景、光线与情绪。禁止英文单词堆砌或英文 tag（人名、商标可保留原文）。
- 一张图只表现一个机位、一个瞬间，禁止把多个镜头竖向或分格拼在同一张图里。
- 多数镜头 beats 用空数组 []。只有一镜内确有多段时间或机位切换时才拆拍；不要把整部戏写成第一镜的 beats。
- 每个 beat 必须有 action：这一拍里人物做什么、怎么动、镜头怎么跟。成片时会把节拍时间轴发给视频模型，video_prompt 只写总体运镜与表演。
- 遵守给出的风格和风格注意事项。
- 从第2镜起，video_prompt 必须写成续写而不是参考：用「将@视频1从最后一帧向后延长」这类写法，必须带 @；禁止写「参考视频1」（参考会变成借鉴生成新片）。只写从上一镜结尾接着发生的增量动作，不要重新开场、不要从静止起势。第1镜不要写承接。@视频1 表示上一镜成片。
不要输出 JSON 以外的文字。`, count, targetSec, models.DramaMaxShots, count)
}

func StoryboardUserPrompt(style, styleNotes, script string, count, targetSec int) string {
	count, targetSec = storyboardBudget(count, targetSec)
	return fmt.Sprintf("风格：%s\n风格注意事项：%s\n目标时长：约 %d 秒\n镜头数：必须输出 %d 个，shots 数组长度必须是 %d，不要只写第 1 镜。\n剧本：\n%s\n\n只输出一个 JSON 对象，不要解释，不要思考过程。", emptyAs(style, "未指定"), emptyAs(styleNotes, "无"), targetSec, count, count, script)
}

func storyboardBudget(count, targetSec int) (int, int) {
	if targetSec <= 0 {
		targetSec = 60
	}
	if count < 1 {
		count = SuggestedShotCount(targetSec)
	}
	if count > models.DramaMaxShots {
		count = models.DramaMaxShots
	}
	return count, targetSec
}

func RewriteImageSystemPrompt() string {
	return "你是竖屏短剧的出图提示词作者。根据成片提示、画面、对白、风格和风格注意事项，写一条给生图模型用的中文画面提示。要求：全程中文，禁止英文单词堆砌或英文 tag（人名、商标可保留）；一条即可；只写一个瞬间、一个机位；强调构图、人物外形一致性、场景与情绪；若现有出图提示是英文，改写成通顺中文；不要把多个镜头拼进一张图；不要解释、不要引号。只输出提示词正文。"
}

func RewriteVideoSystemPrompt(continueFromPrev bool) string {
	s := "你是竖屏短剧的成片提示词作者。根据出图提示、画面、对白、节拍和风格，写一条视频运动与表演提示。要求：中文；写清总体镜头运动、人物动作与情绪；若有节拍，动作必须按拍发生，但不要把节拍时间轴整段抄进提示（出片时会自动附上节拍）；不要把多个机位糊成一句；不要解释、不要引号。只输出提示词正文。"
	if continueFromPrev {
		s += "这一镜要续写上一镜成片。开头写「将@视频1从最后一帧向后延长，承接结尾的姿态、走位与运镜，不要重新开场」，再写从结尾接着发生的动作；必须带 @；禁止写「参考视频1」；不要重新开场。"
	}
	return s
}

func RewriteContinueSystemPrompt() string {
	return "你是竖屏短剧的镜头衔接作者。根据上一镜和本镜的画面、对白、成片提示和节拍，重写成一条能无缝接上的成片提示。要求：全程中文；开头写「将@视频1从最后一帧向后延长，承接结尾的姿态、走位与运镜，不要重新开场」；必须带 @ 符号；禁止写「参考视频1」或「参考上一段」（参考会变成借鉴生成新片而不是续写）；只写从上一镜结尾接着发生的增量动作，不要把本镜写成从静止或全新构图起势；保留本镜该发生的剧情；不要把节拍时间轴整段抄进提示（出片时会自动附上节拍）；不要硬切、不要解释、不要引号。只输出提示词正文。"
}

func RewriteImageUserPrompt(p *models.DramaProject, shot models.DramaShot) string {
	return fmt.Sprintf("成片提示：%s\n画面：%s\n对白：%s\n现有出图提示：%s\n风格：%s\n风格注意事项：%s",
		shot.VideoPrompt, shot.Scene, shot.Dialogue, shot.ImagePrompt, emptyAs(p.Style, "未指定"), emptyAs(p.StyleNotes, "无"))
}

func RewriteVideoUserPrompt(p *models.DramaProject, shot models.DramaShot, continueFromPrev bool) string {
	user := fmt.Sprintf("出图提示：%s\n画面：%s\n对白：%s\n现有成片提示：%s\n节拍：\n%s\n风格：%s\n风格注意事项：%s",
		shot.ImagePrompt, shot.Scene, shot.Dialogue, shot.VideoPrompt, emptyAs(FormatBeats(shot, continueFromPrev), "无"), emptyAs(p.Style, "未指定"), emptyAs(p.StyleNotes, "无"))
	if continueFromPrev {
		user += "\n衔接：必须写成将@视频1从最后一帧向后延长，只写增量动作。"
	}
	return user
}

func RewriteContinueUserPrompt(p *models.DramaProject, prev, shot models.DramaShot) string {
	return fmt.Sprintf("上一镜标题：%s\n上一镜画面：%s\n上一镜对白：%s\n上一镜成片提示：%s\n上一镜节拍：\n%s\n本镜标题：%s\n本镜画面：%s\n本镜对白：%s\n本镜出图提示：%s\n本镜现有成片提示：%s\n本镜节拍：\n%s\n风格：%s\n风格注意事项：%s",
		prev.Title, prev.Scene, prev.Dialogue, prev.VideoPrompt, emptyAs(FormatBeats(prev, false), "无"),
		shot.Title, shot.Scene, shot.Dialogue, shot.ImagePrompt, shot.VideoPrompt, emptyAs(FormatBeats(shot, true), "无"),
		emptyAs(p.Style, "未指定"), emptyAs(p.StyleNotes, "无"))
}

func FormatBeats(shot models.DramaShot, continueFromPrev bool) string {
	if len(shot.Beats) == 0 {
		return ""
	}
	var b strings.Builder
	if continueFromPrev {
		b.WriteString("按节拍依次表演，时间轴从@视频1最后一帧接着数，第一拍不要重新起势、不要从静止开场：")
	} else {
		b.WriteString("按节拍依次表演，时间轴必须对齐，不要把多拍糊成一个动作：")
	}
	n := 0
	for i, beat := range shot.Beats {
		tr := strings.TrimSpace(beat.TimeRange)
		if tr == "" {
			tr = fmt.Sprintf("第%d拍", i+1)
		}
		act := strings.TrimSpace(beat.Action)
		if act == "" {
			act = strings.TrimSpace(beat.ImagePrompt)
		}
		if act == "" {
			continue
		}
		n++
		fmt.Fprintf(&b, "\n%s：%s", tr, act)
	}
	if n == 0 {
		return ""
	}
	return b.String()
}

func ParseStoryboardContent(content string) ([]models.DramaShot, error) {
	text := stripThink(content)
	if m := jsonFence.FindStringSubmatch(text); len(m) > 1 {
		text = strings.TrimSpace(m[1])
	}
	if extracted := extractJSONPayload(text); extracted != "" {
		text = extracted
	}
	shots := ParseShots(text)
	if len(shots) == 0 {
		shots = ParseShots(recoverPartialShots(text))
	}
	if len(shots) == 0 {
		return nil, fmt.Errorf("分镜 JSON 无法解析")
	}
	return DecorateContinue(shots), nil
}

func stripThink(content string) string {
	s := thinkBlock.ReplaceAllString(content, "")
	low := strings.ToLower(s)
	if i := strings.LastIndex(low, "</think>"); i >= 0 {
		return strings.TrimSpace(s[i+len("</think>"):])
	}
	if i := strings.Index(low, "<think>"); i >= 0 {
		rest := s[i+len("<think>"):]
		if j := strings.Index(rest, `{"shots"`); j >= 0 {
			return strings.TrimSpace(rest[j:])
		}
		if j := strings.Index(rest, "{"); j >= 0 {
			return strings.TrimSpace(rest[j:])
		}
		if j := strings.Index(rest, "["); j >= 0 {
			return strings.TrimSpace(rest[j:])
		}
	}
	return strings.TrimSpace(s)
}

func extractJSONPayload(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, `{"shots"`); i >= 0 {
		if extracted := extractBalanced(text[i:]); extracted != "" {
			return extracted
		}
	}
	obj := strings.Index(text, "{")
	arr := strings.Index(text, "[")
	start := -1
	if obj >= 0 && (arr < 0 || obj < arr) {
		start = obj
	} else if arr >= 0 {
		start = arr
	}
	if start < 0 {
		return ""
	}
	return extractBalanced(text[start:])
}

func extractBalanced(text string) string {
	if text == "" {
		return ""
	}
	endChar := byte('}')
	if text[0] == '[' {
		endChar = ']'
	} else if text[0] != '{' {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 && c == endChar {
				return strings.TrimSpace(text[:i+1])
			}
		}
	}
	return ""
}

func recoverPartialShots(text string) string {
	start := -1
	if i := strings.Index(text, `"shots"`); i >= 0 {
		rest := text[i+len(`"shots"`):]
		if j := strings.Index(rest, "["); j >= 0 {
			start = i + len(`"shots"`) + j
		}
	}
	if start < 0 {
		start = strings.Index(text, "[")
	}
	if start < 0 {
		return ""
	}
	objs := extractCompleteArrayObjects(text[start:])
	if len(objs) == 0 {
		return ""
	}
	return "[" + strings.Join(objs, ",") + "]"
}

func extractCompleteArrayObjects(arr string) []string {
	if arr == "" || arr[0] != '[' {
		return nil
	}
	var out []string
	depth := 0
	inStr := false
	esc := false
	objStart := -1
	for i := 0; i < len(arr); i++ {
		c := arr[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			if depth == 1 {
				objStart = i
			}
			depth++
		case '}':
			depth--
			if depth == 1 && objStart >= 0 {
				out = append(out, arr[objStart:i+1])
				objStart = -1
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return out
			}
		}
	}
	return out
}

func DecorateContinue(shots []models.DramaShot) []models.DramaShot {
	shots = NormalizeShots(shots)
	for i := range shots {
		if shots[i].Index <= 1 {
			continue
		}
		shots[i].ContinueFromPrev = true
		shots[i].VideoPrompt = EnsureContinuePrompt(shots[i].VideoPrompt)
	}
	return shots
}

func CleanPromptText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimSpace(s)
	return s
}

func MockScript(idea, style string, targetSec int) string {
	if targetSec <= 0 {
		targetSec = 60
	}
	st := emptyAs(style, "写实口语")
	return fmt.Sprintf(`场1 夜 外 便利店门口
题材：%s
风格：%s
目标约 %d 秒。

角色A站在雨里抽烟，便利店暖光打在玻璃上。
角色B推门出来，两人对视。
A：还是这个点。
B：你也是。
A把烟掐了，让出路。B撑伞走过，没回头。
A看着她的背影，便利店门铃响了一下。`, strings.TrimSpace(idea), st, targetSec)
}

func MockStoryboard(script, style string, n int) []models.DramaShot {
	if n < 1 {
		n = SuggestedShotCount(60)
	}
	parts := splitScriptParts(script, n)
	shots := make([]models.DramaShot, 0, len(parts))
	for i, part := range parts {
		shot := EmptyShot(i + 1)
		shot.Scene = part
		shot.ImagePrompt = part + "。竖屏单机位单瞬间，" + emptyAs(style, "夜雨霓虹")
		shot.VideoPrompt = part
		if i == 0 {
			shot.Dialogue = "还是这个点。"
		}
		if i > 0 {
			shot.ContinueFromPrev = true
			shot.VideoPrompt = EnsureContinuePrompt("接着上一镜的姿态继续：" + part)
		}
		shots = append(shots, shot)
	}
	return shots
}

func MockRewrite(target, source string, continueFromPrev bool) string {
	text := CleanPromptText(source)
	if text == "" {
		text = "竖屏单机位，人物对视，夜雨便利店。"
	}
	switch target {
	case "image_prompt":
		return text + "。中文单瞬间构图，人物外形清晰，不要多机位拼图。"
	case "continue":
		return EnsureContinuePrompt(text)
	default:
		if continueFromPrev {
			return EnsureContinuePrompt(text)
		}
		return text + "。镜头缓慢前推，人物动作连续，对白口型自然。"
	}
}

func splitScriptParts(script string, n int) []string {
	script = strings.TrimSpace(script)
	if n < 1 {
		n = 1
	}
	if n > models.DramaMaxShots {
		n = models.DramaMaxShots
	}
	if script == "" {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("第 %d 镜画面", i+1)
		}
		return out
	}
	blocks := strings.Split(script, "\n")
	var lines []string
	for _, line := range blocks {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = []string{script}
	}
	if len(lines) <= n {
		return lines
	}
	out := make([]string, n)
	size := (len(lines) + n - 1) / n
	for i := 0; i < n; i++ {
		start := i * size
		if start >= len(lines) {
			out[i] = lines[len(lines)-1]
			continue
		}
		end := start + size
		if end > len(lines) {
			end = len(lines)
		}
		out[i] = strings.Join(lines[start:end], " ")
		out[i] = clipRunesPlain(out[i], 80)
	}
	return out
}

func emptyAs(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func clipRunesPlain(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}
