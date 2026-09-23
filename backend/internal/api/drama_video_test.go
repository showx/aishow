package api

import (
	"testing"

	"aishow/internal/drama"
	"aishow/internal/models"
)

func TestPlanDramaVideoDirectorUsesExtraRefs(t *testing.T) {
	p := &models.DramaProject{
		VideoEngine:    models.EngineH3Director,
		ContinueEngine: models.EngineH3Director,
		ImageRefsJSON: drama.EncodeImageRefs([]models.DramaImageRef{
			{UploadID: "char-a", Name: "林晚", Kind: "character"},
			{UploadID: "shop", Name: "便利店", Kind: "scene"},
		}),
	}
	shot := models.DramaShot{ImageUploadID: "shot-img"}
	engine, mode, conds := planDramaVideo(p, shot, nil)
	if engine != models.EngineH3Director || mode != models.ModeRef2VA {
		t.Fatalf("engine=%s mode=%s", engine, mode)
	}
	if !hasUpload(conds, "shot-img") || !hasUpload(conds, "char-a") || !hasUpload(conds, "shop") {
		t.Fatalf("conds=%+v", conds)
	}

	prev := &models.DramaShot{ImageUploadID: "prev-img", VideoUploadID: "prev-vid"}
	shot.ContinueFromPrev = true
	engine, mode, conds = planDramaVideo(p, shot, prev)
	if engine != models.EngineH3Director || mode != models.ModeRef2VA {
		t.Fatalf("continue engine=%s mode=%s", engine, mode)
	}
	if !hasUpload(conds, "prev-vid") || !hasUpload(conds, "shot-img") || !hasUpload(conds, "char-a") {
		t.Fatalf("continue conds=%+v", conds)
	}
}

func TestPlanDramaVideoFirstFrameIgnoresExtraRefs(t *testing.T) {
	p := &models.DramaProject{
		VideoEngine:    models.EngineH3,
		ContinueEngine: models.EngineH3,
		ImageRefsJSON: drama.EncodeImageRefs([]models.DramaImageRef{
			{UploadID: "char-a", Name: "林晚", Kind: "character"},
		}),
	}
	shot := models.DramaShot{ImageUploadID: "shot-img"}
	engine, mode, conds := planDramaVideo(p, shot, nil)
	if engine != models.EngineH3 || mode != models.ModeI2VA {
		t.Fatalf("engine=%s mode=%s", engine, mode)
	}
	if len(conds) != 1 || conds[0].UploadID != "shot-img" || conds[0].Role != "keyframe" {
		t.Fatalf("conds=%+v", conds)
	}
}

func TestPlanDramaVideoDirectorRefsWithoutShotImage(t *testing.T) {
	p := &models.DramaProject{
		VideoEngine: models.EngineH3Director,
		ImageRefsJSON: drama.EncodeImageRefs([]models.DramaImageRef{
			{UploadID: "char-a", Name: "林晚", Kind: "character"},
		}),
	}
	engine, mode, conds := planDramaVideo(p, models.DramaShot{}, nil)
	if engine != models.EngineH3Director || mode != models.ModeRef2VA {
		t.Fatalf("engine=%s mode=%s", engine, mode)
	}
	if len(conds) != 1 || conds[0].UploadID != "char-a" {
		t.Fatalf("conds=%+v", conds)
	}
}

func TestPlanDramaVideoHunyuanKeepsSingleFirstFrame(t *testing.T) {
	p := &models.DramaProject{
		VideoEngine:    models.EngineHunyuanVideo,
		ContinueEngine: models.EngineHunyuanVideo,
	}
	shot := models.DramaShot{ImageUploadID: "shot-img", ContinueFromPrev: true}
	prev := &models.DramaShot{LastFrameUploadID: "last-frame"}
	engine, mode, conds := planDramaVideo(p, shot, prev)
	if engine != models.EngineHunyuanVideo || mode != models.ModeI2VA {
		t.Fatalf("engine=%s mode=%s", engine, mode)
	}
	if len(conds) != 1 || conds[0].UploadID != "last-frame" {
		t.Fatalf("conds=%+v", conds)
	}
}

func hasUpload(conds []models.AssetCondition, id string) bool {
	for _, c := range conds {
		if c.UploadID == id {
			return true
		}
	}
	return false
}
