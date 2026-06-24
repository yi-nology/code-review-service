package runner

import (
	"testing"
)

func TestOpenCodeReviewConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     OpenCodeReviewConfig
		wantErr bool
	}{
		{
			name:    "empty binary should fail",
			cfg:     OpenCodeReviewConfig{Binary: ""},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", TimeoutMinutes: 10, Concurrency: 2},
			wantErr: false,
		},
		{
			name:    "timeout exceeds maximum",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", TimeoutMinutes: 200},
			wantErr: true,
		},
		{
			name:    "concurrency exceeds maximum",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", Concurrency: 64},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
