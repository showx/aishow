package drama

import (
	"encoding/json"
	"strings"

	"aishow/internal/models"
)

func ParseImageRefs(raw string) []models.DramaImageRef {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var refs []models.DramaImageRef
	if json.Unmarshal([]byte(raw), &refs) != nil {
		return nil
	}
	return CleanImageRefs(refs)
}

func CleanImageRefs(refs []models.DramaImageRef) []models.DramaImageRef {
	out := make([]models.DramaImageRef, 0, len(refs))
	seen := map[string]bool{}
	for _, r := range refs {
		id := strings.TrimSpace(r.UploadID)
		if id == "" || seen[id] {
			continue
		}
		kind := strings.TrimSpace(r.Kind)
		if kind != "character" && kind != "scene" {
			kind = ""
		}
		seen[id] = true
		out = append(out, models.DramaImageRef{
			UploadID: id,
			Name:     strings.TrimSpace(r.Name),
			Kind:     kind,
		})
		if len(out) >= models.DramaMaxImageRefs {
			break
		}
	}
	return out
}

func EncodeImageRefs(refs []models.DramaImageRef) string {
	clean := CleanImageRefs(refs)
	if len(clean) == 0 {
		return "[]"
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func MergeImageRefs(p *models.DramaProject, shot models.DramaShot, prev *models.DramaShot) []models.DramaImageRef {
	var out []models.DramaImageRef
	add := func(list []models.DramaImageRef) {
		out = CleanImageRefs(append(out, list...))
	}
	if prev != nil && !shot.SkipPrevImages && strings.TrimSpace(prev.ImageUploadID) != "" {
		add([]models.DramaImageRef{{
			UploadID: prev.ImageUploadID,
			Name:     "上一镜人物",
			Kind:     "character",
		}})
	}
	if p != nil {
		add(ParseImageRefs(p.ImageRefsJSON))
	}
	add(shot.ImageRefs)
	return out
}

func FirstImageRefID(refs []models.DramaImageRef) string {
	for _, r := range refs {
		if strings.TrimSpace(r.UploadID) != "" {
			return r.UploadID
		}
	}
	return ""
}

func FormatImageRefs(refs []models.DramaImageRef) string {
	var chars, scenes []string
	for _, r := range refs {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			continue
		}
		switch r.Kind {
		case "scene":
			scenes = append(scenes, name)
		default:
			chars = append(chars, name)
		}
	}
	var b strings.Builder
	if len(chars) > 0 {
		b.WriteString("人物参考：")
		b.WriteString(strings.Join(chars, "、"))
	}
	if len(scenes) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("场景参考：")
		b.WriteString(strings.Join(scenes, "、"))
	}
	return b.String()
}
