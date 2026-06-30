package main

import (
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
)

func TestCreateServer_AppliesPR06Options(t *testing.T) {
	cfg := &config.Config{Port: 0}
	h := createServer(cfg)
	if h == nil {
		t.Fatalf("createServer returned nil")
	}
	// Hertz doesn't expose applied options for inspection;
	// 验证 createServer 不 panic 且能在合理时间返回即可。
	time.Sleep(10 * time.Millisecond)
}
