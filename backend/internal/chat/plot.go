package chat

import "strings"

var preferredPlotModels = []string{
	"qwen3.5:4b",
	"qwen3.5:7b",
	"qwen2.5:7b",
	"qwen2.5:3b",
	"qwen2.5:14b",
	"qwen2.5:1.5b",
}

func IsPlotModel(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	for _, skip := range []string{"-vl", ":vl", "vl:", "embed", "bge-", "clip", "whisper", "text-encoder", "rerank"} {
		if strings.Contains(n, skip) {
			return false
		}
	}
	return true
}

func PickPlotModel(names []string, preferred string) string {
	byLower := map[string]string{}
	var plot []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		byLower[strings.ToLower(n)] = n
		if IsPlotModel(n) {
			plot = append(plot, n)
		}
	}
	pref := strings.TrimSpace(preferred)
	if pref != "" && IsPlotModel(pref) {
		if actual, ok := byLower[strings.ToLower(pref)]; ok {
			return actual
		}
		return pref
	}
	for _, p := range preferredPlotModels {
		if actual, ok := byLower[p]; ok {
			return actual
		}
		for _, n := range plot {
			if strings.HasPrefix(strings.ToLower(n), p) {
				return n
			}
		}
	}
	if len(plot) > 0 {
		return plot[0]
	}
	if pref != "" {
		return pref
	}
	if len(names) > 0 {
		return strings.TrimSpace(names[0])
	}
	return ""
}

func RankPlotModels(names []string, preferred string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	add(PickPlotModel(names, preferred))
	for _, n := range names {
		if IsPlotModel(n) {
			add(n)
		}
	}
	for _, n := range names {
		add(n)
	}
	return out
}
