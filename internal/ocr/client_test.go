package ocr

import (
	"testing"

	"github.com/oekit/concurrency/async"
)

func TestClient_NewAndCfg(t *testing.T) {
	cfg := Defaults().ToConfig()
	cfg.Binary = "test-ocr"

	c := NewClient(cfg)
	if got := c.Cfg().Binary; got != "test-ocr" {
		t.Errorf("expected Binary=test-ocr, got %q", got)
	}
}

func TestClient_Refresh(t *testing.T) {
	c := NewClient(Defaults().ToConfig())
	if c.WasRefreshed() {
		t.Fatalf("expected not refreshed initially")
	}

	cfg2 := Defaults().ToConfig()
	cfg2.Binary = "refreshed"
	c.Refresh(cfg2)

	if !c.WasRefreshed() {
		t.Errorf("expected WasRefreshed=true after Refresh")
	}
	if got := c.Cfg().Binary; got != "refreshed" {
		t.Errorf("expected Binary=refreshed after Refresh, got %q", got)
	}
}

func TestClient_ConcurrentAccess(t *testing.T) {
	c := NewClient(Defaults().ToConfig())
	done := make(chan struct{})
	async.GoSafe(func() {
		for i := 0; i < 100; i++ {
			_ = c.Cfg()
		}
		close(done)
	})
	for i := 0; i < 100; i++ {
		cfg := Defaults().ToConfig()
		cfg.Binary = "x"
		c.Refresh(cfg)
	}
	<-done
}
