package domain

import "testing"

func TestEffectiveGateMode(t *testing.T) {
	tests := []struct {
		name     string
		gateMode string
		engine   string
		want     string
	}{
		{name: "empty follows ocr default strong", gateMode: "", engine: EngineOCR, want: GateModeStrong},
		{name: "empty follows argus default weak", gateMode: "", engine: EngineArgus, want: GateModeWeak},
		{name: "unknown value follows engine default", gateMode: "blocking", engine: EngineArgus, want: GateModeWeak},
		{name: "explicit strong wins over argus default", gateMode: "strong", engine: EngineArgus, want: GateModeStrong},
		{name: "explicit weak wins over ocr default", gateMode: "weak", engine: EngineOCR, want: GateModeWeak},
		{name: "case and space normalized", gateMode: " Weak ", engine: EngineOCR, want: GateModeWeak},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveGateMode(tt.gateMode, tt.engine); got != tt.want {
				t.Errorf("EffectiveGateMode(%q, %q) = %q, want %q", tt.gateMode, tt.engine, got, tt.want)
			}
		})
	}
}

func TestWeakGateDescKeepsPrefixVisible(t *testing.T) {
	desc := WeakGateDesc("argus 裁决：要求修改")
	if len(WeakGatePrefix) == 0 || desc[:len(WeakGatePrefix)] != WeakGatePrefix {
		t.Fatalf("WeakGateDesc should start with prefix %q, got %q", WeakGatePrefix, desc)
	}
}
