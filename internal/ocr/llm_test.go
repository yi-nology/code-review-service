package ocr

import "testing"

func TestErrNotConfigured(t *testing.T) {
	if ErrNotConfigured.Error() != "llm not configured or disabled" {
		t.Errorf("unexpected message: %v", ErrNotConfigured)
	}
}
