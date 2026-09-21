package tts

import "testing"

func TestSpeechURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:8880":                 "http://127.0.0.1:8880/v1/audio/speech",
		"http://127.0.0.1:8880/v1":              "http://127.0.0.1:8880/v1/audio/speech",
		"http://127.0.0.1:8880/v1/audio/speech": "http://127.0.0.1:8880/v1/audio/speech",
		"":                                      "",
	}
	for in, want := range cases {
		if got := SpeechURL(in); got != want {
			t.Fatalf("SpeechURL(%q)=%q want %q", in, got, want)
		}
	}
}
