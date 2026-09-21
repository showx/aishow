package chat

import "testing"

func TestCompletionsURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:11434":                    "http://127.0.0.1:11434/v1/chat/completions",
		"http://127.0.0.1:11434/":                   "http://127.0.0.1:11434/v1/chat/completions",
		"http://127.0.0.1:11434/v1":                 "http://127.0.0.1:11434/v1/chat/completions",
		"http://127.0.0.1:8080/v1/chat/completions": "http://127.0.0.1:8080/v1/chat/completions",
		"": "",
	}
	for in, want := range cases {
		if got := CompletionsURL(in); got != want {
			t.Fatalf("CompletionsURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBaseURL(t *testing.T) {
	got := BaseURL("http://127.0.0.1:11434/v1/chat/completions")
	if got != "http://127.0.0.1:11434" {
		t.Fatalf("got %q", got)
	}
}

func TestPickPlotModelSkipsVL(t *testing.T) {
	list := []string{"qwen3-vl:8b", "bge-m3:latest", "qwen3.5:4b", "deepseek-r1:8b"}
	if got := PickPlotModel(list, ""); got != "qwen3.5:4b" {
		t.Fatalf("got %q", got)
	}
	if got := PickPlotModel(list, "qwen3-vl:8b"); got != "qwen3.5:4b" {
		t.Fatalf("preferred vl should be skipped: %q", got)
	}
	if IsPlotModel("qwen3-vl:8b") || IsPlotModel("bge-m3") {
		t.Fatal("vl/embed should not be plot models")
	}
}
