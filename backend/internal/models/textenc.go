package models

import "strings"

const PromptRewriterContextIR = "H3-Context-IR"

type SidecarHealth struct {
	Ready            bool
	LatencyMS        int64
	Detail           string
	TextEncoder      string
	TextEncoderLabel string
}

func ParseSidecarHealth(body map[string]any, lat int64, reachable bool) SidecarHealth {
	out := SidecarHealth{LatencyMS: lat}
	if !reachable || body == nil {
		out.Detail = "离线"
		return out
	}
	if err, _ := body["error"].(string); strings.TrimSpace(err) != "" {
		out.Detail = err
		out.TextEncoder = sidecarString(body, "text_encoder")
		out.TextEncoderLabel = sidecarString(body, "text_encoder_label")
		return out
	}
	out.TextEncoder = sidecarString(body, "text_encoder")
	out.TextEncoderLabel = sidecarString(body, "text_encoder_label")
	if flag, exists := body["ready"]; exists {
		if b, ok := flag.(bool); ok {
			out.Ready = b
			if b {
				out.Detail = "已加载"
				return out
			}
			if loading, _ := body["loading"].(bool); loading {
				out.Detail = "加载中"
				return out
			}
			out.Detail = "进程在，模型未就绪"
			return out
		}
	}
	out.Ready = true
	out.Detail = "HTTP 可达"
	return out
}

func sidecarString(body map[string]any, key string) string {
	v, _ := body[key].(string)
	return strings.TrimSpace(v)
}

func TextEncoderBasename(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, `\`, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func PromptRewriterFor(engine string, enhance bool) string {
	if enhance && !IsImageEngine(engine) {
		return PromptRewriterContextIR
	}
	return ""
}

func DescribeTextEncoder(engine, filename string) (id, label string) {
	name := TextEncoderBasename(filename)
	low := strings.ToLower(name)
	switch {
	case strings.Contains(low, "qwen3"):
		lab := qwen3Label(low)
		if IsH3PinkCherryInt8(engine) {
			return name, "PinkCherry " + lab
		}
		return name, lab
	case strings.Contains(low, "text-encoder") || strings.Contains(low, "text_encoder"):
		if strings.Contains(low, "nf4") {
			return name, "Qwen3-VL 32B NF4"
		}
		return name, "Qwen3-VL 32B"
	case strings.Contains(low, "llada"):
		if strings.Contains(low, "turbo") {
			return name, "LLaDA-Image 6B Turbo"
		}
		return name, "LLaDA-Image 6B"
	}
	switch CanonicalEngine(engine) {
	case EngineFastH3, EngineH3Ref2VAInt8, EngineH3Director:
		if name == "" {
			return "qwen3vl-32b", "Qwen3-VL 32B 量化"
		}
		if label := qwen3Label(low); label != "Qwen3-VL 32B" || strings.Contains(low, "qwen") {
			return name, label
		}
		return name, "Qwen3-VL 32B 量化"
	case EngineH3PinkCherryInt8:
		if name == "" {
			return "pinkcherry-qwen3vl-32b", "PinkCherry Qwen3-VL 32B"
		}
		if label := qwen3Label(low); label != "Qwen3-VL 32B" || strings.Contains(low, "qwen") {
			return name, "PinkCherry " + label
		}
		return name, "PinkCherry Qwen3-VL 32B"
	case EngineLLadaImage:
		if name == "" {
			return "LLaDA-Image", "LLaDA-Image 6B"
		}
		if strings.Contains(low, "turbo") {
			return name, "LLaDA-Image 6B Turbo"
		}
		return name, "LLaDA-Image 6B"
	default:
		if name == "" {
			return "minimax-h3-text-encoder-nf4.safetensors", "Qwen3-VL 32B NF4"
		}
		if strings.Contains(low, "nf4") {
			return name, "Qwen3-VL 32B NF4"
		}
		return name, "Qwen3-VL 32B"
	}
}

func qwen3Label(low string) string {
	switch {
	case strings.Contains(low, "nvfp4"):
		return "Qwen3-VL 32B NVFP4"
	case strings.Contains(low, "int8"):
		return "Qwen3-VL 32B INT8"
	case strings.Contains(low, "nf4"):
		return "Qwen3-VL 32B NF4"
	default:
		return "Qwen3-VL 32B"
	}
}
