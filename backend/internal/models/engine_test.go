package models

import "testing"

func TestCanonicalEngine(t *testing.T) {
	cases := map[string]string{
		"h3":              EngineH3,
		"h3-base":         EngineH3,
		"fasth3":          EngineFastH3,
		"h3-max":          EngineFastH3,
		"h3-ref2va-int8":  EngineH3Ref2VAInt8,
		"ref2va-int8":     EngineH3Ref2VAInt8,
		"llada-image":     EngineLLadaImage,
		"llada":           EngineLLadaImage,
	}
	for in, want := range cases {
		if got := CanonicalEngine(in); got != want {
			t.Fatalf("CanonicalEngine(%q)=%q want %q", in, got, want)
		}
	}
}

func TestEngineAliasesIncludeCanonical(t *testing.T) {
	for _, id := range []string{EngineH3, EngineFastH3, EngineH3Ref2VAInt8, EngineLLadaImage} {
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
