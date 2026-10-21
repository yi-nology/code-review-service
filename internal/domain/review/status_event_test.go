package domain

import "testing"

func TestApplyWeakGate(t *testing.T) {
	tests := []struct {
		name                string
		gateMode, engine    string
		strongState         string
		description         string
		wantState, wantDesc string
	}{
		{"explicit weak", "weak", "ocr", StateFailed, "门禁未过", StateSuccess, WeakGateDesc("门禁未过")},
		{"explicit strong", "strong", "ocr", StateFailed, "门禁未过", StateFailed, "门禁未过"},
		{"argus defaults weak", "", "argus", StateError, "failed", StateSuccess, WeakGateDesc("failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState, gotDesc := ApplyWeakGate(tt.gateMode, tt.engine, tt.strongState, tt.description)
			if gotState != tt.wantState || gotDesc != tt.wantDesc {
				t.Fatalf("got (%q,%q), want (%q,%q)", gotState, gotDesc, tt.wantState, tt.wantDesc)
			}
		})
	}
}
