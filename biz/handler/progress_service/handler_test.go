package progress_service

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/progress"
)

func TestNewProgressHandler(t *testing.T) {
	pm := progress.NewProgressManager()
	h := NewProgressHandler(pm)
	if h == nil {
		t.Fatalf("NewProgressHandler returned nil")
	}
	if h.progressManager != pm {
		t.Fatalf("expected progressManager to be set")
	}
	if h.upgrader.CheckOrigin == nil {
		t.Fatalf("expected upgrader.CheckOrigin to be set")
	}
}

func TestInitHandler_BackwardCompat(t *testing.T) {
	pm := progress.NewProgressManager()
	InitHandler(pm)
	if handlerInst == nil {
		t.Fatalf("expected handlerInst to be set after InitHandler")
	}
	if handlerInst.progressManager != pm {
		t.Fatalf("expected progressManager to be set on handlerInst")
	}
}
