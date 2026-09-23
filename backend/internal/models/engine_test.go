package models

import "testing"

func TestCanonicalEngine(t *testing.T) {
	cases := map[string]string{
		"h3":                 EngineH3,
		"h3-base":            EngineH3,
		"fasth3":             EngineFastH3,
		"h3-max":             EngineFastH3,
		"h3-turbo":           EngineH3Turbo,
		"h3-turbo-lora":      EngineH3Turbo,
		"h3-ref2va-int8":     EngineH3Ref2VAInt8,
		"ref2va-int8":        EngineH3Ref2VAInt8,
		"h3-pinkcherry-int8": EngineH3PinkCherryInt8,
		"pinkcherry":         EngineH3PinkCherryInt8,
		"h3-director":        EngineH3Director,
		"timeline-director":  EngineH3Director,
		"llada-image":        EngineLLadaImage,
		"llada":              EngineLLadaImage,
		"qwen-image":         EngineQwenImage,
		"qwen-image-2.1":     EngineQwenImage,
		"hunyuan-video":      EngineHunyuanVideo,
		"hunyuan":            EngineHunyuanVideo,
		"ltx-2.3":            EngineLTX23,
		"ltx23":              EngineLTX23,
	}
	for in, want := range cases {
		if got := CanonicalEngine(in); got != want {
			t.Fatalf("CanonicalEngine(%q)=%q want %q", in, got, want)
		}
	}
}

func TestEngineAliasesIncludeCanonical(t *testing.T) {
	for _, id := range []string{EngineH3, EngineFastH3, EngineH3Turbo, EngineH3Ref2VAInt8, EngineH3PinkCherryInt8, EngineH3Director, EngineLLadaImage, EngineQwenImage, EngineHunyuanVideo, EngineLTX23} {
		found := false
		for _, a := range EngineAliases(id) {
			if a == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("EngineAliases(%q) missing canonical id", id)
		}
	}
}

func TestSupportsRef2VAAndFirstFrame(t *testing.T) {
	if !SupportsRef2VA(EngineH3Ref2VAInt8) || !SupportsRef2VA(EngineH3Director) {
		t.Fatal("ref2va engines")
	}
	if SupportsRef2VA(EngineH3) || SupportsRef2VA(EngineLLadaImage) || SupportsRef2VA(EngineQwenImage) {
		t.Fatal("h3/llada/qwen should not advertise ref2va")
	}
	if !SupportsFirstFrame(EngineH3) || !SupportsFirstFrame(EngineH3Turbo) || !SupportsFirstFrame(EngineH3PinkCherryInt8) || !SupportsFirstFrame(EngineHunyuanVideo) || !SupportsFirstFrame(EngineLTX23) {
		t.Fatal("first-frame engines")
	}
	if !SupportsBridge(EngineH3) || SupportsBridge(EngineHunyuanVideo) || SupportsBridge(EngineLTX23) {
		t.Fatal("bridge is only h3 first-last engines")
	}
	if SupportsFirstFrame(EngineFastH3) || SupportsFirstFrame(EngineH3Ref2VAInt8) {
		t.Fatal("fasth3/ref2va are not first-frame engines")
	}
	if !ContinueUsesLastFrame(EngineH3) || ContinueUsesLastFrame(EngineH3Ref2VAInt8) {
		t.Fatal("last-frame continue")
	}
}
