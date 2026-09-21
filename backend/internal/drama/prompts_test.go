package drama

import (
	"strings"
	"testing"

	"aishow/internal/models"
)

func TestParseStoryboardContentObjectAndFence(t *testing.T) {
	raw := `{"shots":[{"title":"巷口","scene":"雨夜便利店","image_prompt":"竖屏单瞬间","video_prompt":"人物走来","duration":5},{"title":"对视","scene":"两人停住","image_prompt":"近景","video_prompt":"向前走","duration":4}]}`
	shots, err := ParseStoryboardContent("```json\n" + raw + "\n```")
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 {
		t.Fatalf("len=%d", len(shots))
	}
	if shots[0].Index != 1 || shots[1].Index != 2 {
		t.Fatalf("indexes %d %d", shots[0].Index, shots[1].Index)
	}
	if !shots[1].ContinueFromPrev {
		t.Fatal("second shot should continue")
	}
	if !strings.Contains(shots[1].VideoPrompt, "@视频1") {
		t.Fatalf("continue cue missing: %q", shots[1].VideoPrompt)
	}
}

func TestParseStoryboardContentPrefersShotsObject(t *testing.T) {
	raw := `先看这一镜 {"index":1,"title":"示例","scene":"不要用这个"} 正式结果：{"shots":[{"scene":"巷口","image_prompt":"a"},{"scene":"便利店","image_prompt":"b"}]}`
	shots, err := ParseStoryboardContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 || shots[0].Scene != "巷口" || shots[1].Scene != "便利店" {
		t.Fatalf("%+v", shots)
	}
}

func TestParseStoryboardContentTruncatedKeepsCompleteShots(t *testing.T) {
	raw := `{"shots":[{"title":"巷口","scene":"雨夜","image_prompt":"竖屏"},{"title":"对视","scene":"停住","image_prompt":"近景"},{"title":"离开","scene":"`
	shots, err := ParseStoryboardContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 {
		t.Fatalf("len=%d %+v", len(shots), shots)
	}
}

func TestParseStoryboardContentStripsUnclosedThink(t *testing.T) {
	raw := `<think>草稿 {"index":1,"scene":"假的"} {"shots":[{"scene":"雨夜","image_prompt":"竖屏"},{"scene":"便利店","image_prompt":"近景"}]}`
	shots, err := ParseStoryboardContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 || shots[0].Scene != "雨夜" {
		t.Fatalf("%+v", shots)
	}
}

func TestStoryboardPromptAsksForCount(t *testing.T) {
	sys := StoryboardSystemPrompt(6, 30)
	if !strings.Contains(sys, "6 个镜头") || !strings.Contains(sys, "禁止只返回第 1 镜") {
		t.Fatalf("system=%s", sys)
	}
	if !strings.Contains(sys, `"index":2`) {
		t.Fatal("example should include a second shot")
	}
	user := StoryboardUserPrompt("夜雨", "", "对白", 6, 30)
	if !strings.Contains(user, "必须输出 6 个") || !strings.Contains(user, "目标时长：约 30 秒") {
		t.Fatalf("user=%s", user)
	}
}

func TestParseStoryboardContentArray(t *testing.T) {
	shots, err := ParseStoryboardContent(`[{"scene":"A","image_prompt":"a"},{"scene":"B","image_prompt":"b"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 || shots[0].Scene != "A" {
		t.Fatalf("%+v", shots)
	}
}

func TestParseStoryboardContentStripsThink(t *testing.T) {
	raw := "<think>先拆两镜</think>\n说明如下\n{\"shots\":[{\"scene\":\"雨夜\",\"image_prompt\":\"竖屏\"}]}"
	shots, err := ParseStoryboardContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 1 || shots[0].Scene != "雨夜" {
		t.Fatalf("%+v", shots)
	}
}

func TestParseStoryboardContentRejectsGarbage(t *testing.T) {
	if _, err := ParseStoryboardContent("这不是 JSON"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecorateContinueFirstShotUntouched(t *testing.T) {
	shots := DecorateContinue([]models.DramaShot{
		{Scene: "开场", VideoPrompt: "缓推"},
		{Scene: "离开", VideoPrompt: "参考视频1 继续走"},
	})
	if shots[0].ContinueFromPrev {
		t.Fatal("first shot should not continue")
	}
	if strings.Contains(shots[0].VideoPrompt, ContinueCue) {
		t.Fatalf("first shot prompt rewritten: %q", shots[0].VideoPrompt)
	}
	if !shots[1].ContinueFromPrev {
		t.Fatal("second shot should continue")
	}
	if !strings.Contains(shots[1].VideoPrompt, "@视频1") {
		t.Fatalf("should rewrite 参考视频1: %q", shots[1].VideoPrompt)
	}
}

func TestMockRewrite(t *testing.T) {
	img := MockRewrite("image_prompt", "夜雨对视", false)
	if !strings.Contains(img, "夜雨对视") || !strings.Contains(img, "单瞬间") {
		t.Fatalf("image=%q", img)
	}
	cont := MockRewrite("continue", "转身离开", true)
	if !strings.HasPrefix(cont, ContinueCue[:9]) {
		t.Fatalf("continue=%q", cont)
	}
}

func TestCleanPromptText(t *testing.T) {
	if got := CleanPromptText("  \"hello\"  "); got != "hello" {
		t.Fatalf("got %q", got)
	}
}
