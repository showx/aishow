package models

import "testing"

func TestDescribeTextEncoder(t *testing.T) {
	cases := []struct {
		engine, file, id, label string
	}{
		{"h3", "", "minimax-h3-text-encoder-nf4.safetensors", "Qwen3-VL 32B NF4"},
		{"h3", `E:\MiniMax-H3\models\minimax-h3-text-encoder-nf4.safetensors`, "minimax-h3-text-encoder-nf4.safetensors", "Qwen3-VL 32B NF4"},
		{"h3-turbo", "", "minimax-h3-text-encoder-nf4.safetensors", "Qwen3-VL 32B NF4"},
		{"fasth3", "qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors", "qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors", "Qwen3-VL 32B NVFP4"},
		{"h3-ref2va-int8", "qwen3vl_32b_minimax_h3_int8_convrot.safetensors", "qwen3vl_32b_minimax_h3_int8_convrot.safetensors", "Qwen3-VL 32B INT8"},
		{"h3-director", "", "qwen3vl-32b", "Qwen3-VL 32B 量化"},
		{"h3-pinkcherry-int8", "", "pinkcherry-qwen3vl-32b", "PinkCherry Qwen3-VL 32B"},
		{"h3-pinkcherry-int8", "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors", "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors", "PinkCherry Qwen3-VL 32B NVFP4"},
		{"fasth3", "", "qwen3vl-32b", "Qwen3-VL 32B 量化"},
		{"llada-image", "LLaDA-Image-Turbo", "LLaDA-Image-Turbo", "LLaDA-Image 6B Turbo"},
		{"llada-image", "", "LLaDA-Image", "LLaDA-Image 6B"},
		{"qwen-image", "", "Qwen-Image-2.1", "Qwen-Image-2.1"},
		{"qwen-image", "Qwen-Image-2.1", "Qwen-Image-2.1", "Qwen-Image-2.1"},
		{"hunyuan-video", "", "Qwen2.5-VL-7B", "HunyuanVideo-1.5 Qwen2.5-VL"},
		{"ltx-2.3", "", "Gemma-3-12B", "LTX-2.3 Gemma 3"},
	}
	for _, c := range cases {
		id, label := DescribeTextEncoder(c.engine, c.file)
		if id != c.id || label != c.label {
			t.Fatalf("DescribeTextEncoder(%q, %q)=(%q, %q) want (%q, %q)", c.engine, c.file, id, label, c.id, c.label)
		}
	}
}

func TestPromptRewriterFor(t *testing.T) {
	if got := PromptRewriterFor("h3", true); got != PromptRewriterContextIR {
		t.Fatalf("h3 enhance: %q", got)
	}
	if got := PromptRewriterFor("llada-image", true); got != "" {
		t.Fatalf("llada should not rewrite: %q", got)
	}
	if got := PromptRewriterFor("qwen-image", true); got != "" {
		t.Fatalf("qwen should not rewrite: %q", got)
	}
	if got := PromptRewriterFor("hunyuan-video", true); got != "" {
		t.Fatalf("hunyuan should not rewrite: %q", got)
	}
	if got := PromptRewriterFor("ltx-2.3", true); got != "" {
		t.Fatalf("ltx should not rewrite: %q", got)
	}
}

func TestParseSidecarHealth(t *testing.T) {
	h := ParseSidecarHealth(map[string]any{
		"ok":                 true,
		"ready":              true,
		"text_encoder":       "qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors",
		"text_encoder_label": "Qwen3-VL 32B NVFP4",
	}, 12, true)
	if !h.Ready || h.TextEncoderLabel != "Qwen3-VL 32B NVFP4" {
		t.Fatalf("%+v", h)
	}
	offline := ParseSidecarHealth(nil, 3, false)
	if offline.Ready || offline.Detail != "离线" {
		t.Fatalf("%+v", offline)
	}
}
