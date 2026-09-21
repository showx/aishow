package drama

import (
	"strings"
	"testing"

	"aishow/internal/models"
)

func TestCleanAndMergeImageRefs(t *testing.T) {
	raw := `[{"upload_id":"a","name":"林晚","kind":"character"},{"upload_id":"a","name":"重复"},{"upload_id":"b","name":"便利店","kind":"scene"}]`
	refs := ParseImageRefs(raw)
	if len(refs) != 2 {
		t.Fatalf("len=%d", len(refs))
	}
	p := &models.DramaProject{ImageRefsJSON: EncodeImageRefs(refs)}
	prev := &models.DramaShot{ImageUploadID: "prev"}
	shot := models.DramaShot{SkipPrevImages: false}
	got := MergeImageRefs(p, shot, prev)
	if FirstImageRefID(got) != "prev" {
		t.Fatalf("first=%q", FirstImageRefID(got))
	}
	note := FormatImageRefs(got)
	if !strings.Contains(note, "林晚") || !strings.Contains(note, "便利店") {
		t.Fatalf("note=%q", note)
	}
	shot.SkipPrevImages = true
	got = MergeImageRefs(p, shot, prev)
	if FirstImageRefID(got) != "a" {
		t.Fatalf("skip prev first=%q", FirstImageRefID(got))
	}
}

func TestBuildSubtitleCuesAndASS(t *testing.T) {
	shots := []models.DramaShotView{
		{DramaShot: models.DramaShot{Index: 1, Duration: 5, Dialogue: "还是这个点。"}},
		{DramaShot: models.DramaShot{Index: 2, Duration: 4, Dialogue: "你也是。"}},
	}
	cues := BuildSubtitleCues(shots, []int{1, 2})
	if len(cues) != 2 || cues[1].Start != 5 || cues[1].End != 9 {
		t.Fatalf("%+v", cues)
	}
	ass := WriteASS(cues, 480, 854)
	if !strings.Contains(ass, "还是这个点。") || !strings.Contains(ass, "0:00:05.00") {
		t.Fatalf("ass=%s", ass)
	}
	if got := assTime(0); got != "0:00:00.00" {
		t.Fatalf("t0=%s", got)
	}
}
