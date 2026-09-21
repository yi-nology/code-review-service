package domain

import "testing"

func TestEffectiveEngine(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", EngineOCR},        // 历史 OCR 记录：request JSONB 无 engine 键
		{"ocr", EngineOCR},     // 显式 ocr
		{"argus", EngineArgus}, // 显式 argus
		{"ARGUS", EngineArgus}, // 大小写归一
		{" argus ", EngineArgus},
		{"gpt", EngineOCR}, // 未知值（历史脏数据）按 OCR 处理
	}
	for _, c := range cases {
		if got := EffectiveEngine(c.in); got != c.want {
			t.Errorf("EffectiveEngine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsValidEngineFilter(t *testing.T) {
	for _, v := range []string{"", "ocr", "argus", " OCR ", "Argus"} {
		if !IsValidEngineFilter(v) {
			t.Errorf("IsValidEngineFilter(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"gpt", "claude", "ocr argus"} {
		if IsValidEngineFilter(v) {
			t.Errorf("IsValidEngineFilter(%q) = true, want false", v)
		}
	}
}
