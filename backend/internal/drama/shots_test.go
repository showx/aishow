package drama

import (
	"testing"

	"aishow/internal/models"
)

func TestNormalizeShotsReindexes(t *testing.T) {
	got := NormalizeShots(ParseShots(`[{"title":"A","scene":"巷口","duration":1},{"image_prompt":"特写","duration":99}]`))
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Index != 1 || got[1].Index != 2 {
		t.Fatalf("indexes %d %d", got[0].Index, got[1].Index)
	}
	if got[0].Duration != 2 {
		t.Fatalf("min duration=%v", got[0].Duration)
	}
	if got[1].Duration != 30 {
		t.Fatalf("max duration=%v", got[1].Duration)
	}
	if got[1].Title != "第 2 镜" {
		t.Fatalf("title=%q", got[1].Title)
	}
}

func TestEnsureContinuePrompt(t *testing.T) {
	got := EnsureContinuePrompt("人物向前走")
	if got[:len(ContinueCue)] != ContinueCue {
		t.Fatalf("missing cue: %q", got)
	}
	again := EnsureContinuePrompt(got)
	if again != got {
		t.Fatalf("should be idempotent")
	}
	fixed := EnsureContinuePrompt("参考视频1 继续走")
	if !containsAll(fixed, "@视频1", ContinueCue[:9]) {
		t.Fatalf("should rewrite 参考视频1: %q", fixed)
	}
}

func TestImageAndVideoPrompt(t *testing.T) {
	p := ImagePrompt(nil, "巷口雨夜", false)
	if p == "巷口雨夜" || !containsAll(p, "巷口雨夜", "一个瞬间") {
		t.Fatalf("image prompt=%q", p)
	}
	shot := EmptyShot(2)
	shot.Scene = "转身离开"
	shot.Dialogue = "走吧"
	shot.ContinueFromPrev = true
	v := VideoPrompt(shot, "胶片颗粒")
	if !containsAll(v, ContinueCue, "对白：走吧", "胶片颗粒") {
		t.Fatalf("video prompt=%q", v)
	}
}

func TestGates(t *testing.T) {
	if RequireScript("  ") == nil {
		t.Fatal("empty script")
	}
	if RequireStoryboard(nil) == nil {
		t.Fatal("empty shots")
	}
	shots := FillEmptyShots(2)
	if RequireStoryboard(shots) == nil {
		t.Fatal("empty scene should fail")
	}
	shots[0].Scene = "A"
	shots[1].ImagePrompt = "B"
	if err := RequireStoryboard(shots); err != nil {
		t.Fatal(err)
	}
}

func TestRequireStepVideoDoesNotWaitForAllImages(t *testing.T) {
	shots := FillEmptyShots(2)
	shots[0].Scene = "巷口"
	shots[1].Scene = "便利店"
	if err := RequireStep(models.DramaStepVideo, "剧本", shots, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileSize(t *testing.T) {
	w, h := CompileSize("9:16", 480)
	if w != 480 || h != 854 {
		t.Fatalf("9:16 480 -> %dx%d", w, h)
	}
	w, h = CompileSize("16:9", 480)
	if w != 854 || h != 480 {
		t.Fatalf("16:9 480 -> %dx%d", w, h)
	}
}

func TestSuggestedShotCount(t *testing.T) {
	if n := SuggestedShotCount(60); n != 8 {
		t.Fatalf("60s -> %d", n)
	}
	if SuggestedShotCount(10) != 2 {
		t.Fatalf("10s -> %d", SuggestedShotCount(10))
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if p == "" {
			continue
		}
		if !stringContains(s, p) {
			return false
		}
	}
	return true
}

func stringContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
