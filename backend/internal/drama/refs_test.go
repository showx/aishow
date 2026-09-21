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

func TestVideoImageRefsOrderAndCap(t *testing.T) {
	var extras []models.DramaImageRef
	for i := 0; i < 12; i++ {
		extras = append(extras, models.DramaImageRef{
			UploadID: string(rune('A' + i)),
			Name:     "角色" + string(rune('A'+i)),
			Kind:     "character",
		})
	}
	p := &models.DramaProject{ImageRefsJSON: EncodeImageRefs(extras)}
	shot := models.DramaShot{ImageUploadID: "shot-img"}
	prev := &models.DramaShot{ImageUploadID: "prev-img"}
	got := VideoImageRefs(p, shot, prev)
	if len(got) != models.DramaMaxVideoImageRefs {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].UploadID != "shot-img" {
		t.Fatalf("first=%q", got[0].UploadID)
	}
	for _, r := range got {
		if r.UploadID == "prev-img" {
			t.Fatal("prev still should lose to extra refs when cap is full")
		}
	}
	few := &models.DramaProject{ImageRefsJSON: `[{"upload_id":"char-a","name":"林晚","kind":"character"}]`}
	got = VideoImageRefs(few, models.DramaShot{ImageUploadID: "shot-img"}, prev)
	if len(got) != 3 || got[2].UploadID != "prev-img" {
		t.Fatalf("with room prev should stay: %+v", got)
	}
	shot.SkipPrevImages = true
	got = VideoImageRefs(p, models.DramaShot{ImageUploadID: "shot-img", SkipPrevImages: true, ImageRefs: []models.DramaImageRef{{UploadID: "shot-extra", Name: "特写", Kind: "character"}}}, prev)
	if got[0].UploadID != "shot-img" || got[1].UploadID != "shot-extra" {
		t.Fatalf("order %+v", got)
	}
	note := FormatVideoImageNote(got)
	if !strings.Contains(note, "特写") || !strings.Contains(note, "保持一致") {
		t.Fatalf("note=%q", note)
	}
	conds := VideoImageConditions(p, models.DramaShot{ImageUploadID: "shot-img"}, nil)
	if len(conds) < 2 || conds[0].UploadID != "shot-img" || conds[0].Role != "reference" || conds[0].Type != "image" {
		t.Fatalf("conds=%+v", conds)
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
