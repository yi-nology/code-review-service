package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/oekit/logx"
)

const (
	defaultInterval = 1 * time.Hour
	defaultPattern  = "review-workspace-*"
)

type WorkspaceCleaner struct {
	baseDir    string
	pattern    string
	maxAge     time.Duration
	interval   time.Duration
	scanTicker *time.Ticker
	stopCh     chan struct{}
	doneCh     chan struct{}
}

type Option func(*WorkspaceCleaner)

func WithInterval(interval time.Duration) Option {
	return func(c *WorkspaceCleaner) {
		if interval > 0 {
			c.interval = interval
		}
	}
}

func NewWorkspaceCleaner(maxAge time.Duration, opts ...Option) *WorkspaceCleaner {
	c := &WorkspaceCleaner{
		baseDir:  os.TempDir(),
		pattern:  defaultPattern,
		maxAge:   maxAge,
		interval: defaultInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start 启动清理协程。PR-04 修复：
// 旧实现会同步执行一次 cleanup()，阻塞 fx OnStart，/tmp 巨大时启动慢；
// 新实现首次清理也放到 goroutine 中，Start 立即返回。
func (c *WorkspaceCleaner) Start() {
	logx.Infof("starting workspace cleaner, baseDir=%s, pattern=%s, maxAge=%v, interval=%v",
		c.baseDir, c.pattern, c.maxAge, c.interval)

	c.scanTicker = time.NewTicker(c.interval)
	go c.loop()
}

func (c *WorkspaceCleaner) loop() {
	defer close(c.doneCh)
	for {
		select {
		case <-c.scanTicker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

func (c *WorkspaceCleaner) cleanup() {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Warnf("workspace cleaner: failed to read dir %s: %v", c.baseDir, err)
		}
		return
	}

	cutoff := time.Now().Add(-c.maxAge)
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			matched, _ := filepath.Match(c.pattern, entry.Name())
			if !matched {
				continue
			}
		} else {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			dirPath := filepath.Join(c.baseDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				logx.Warnf("workspace cleaner: failed to remove %s: %v", dirPath, err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		logx.Infof("workspace cleaner: removed %d expired workspace(s)", cleaned)
	}
}

// Stop 停止清理协程。PR-04 修复：阻塞直到 goroutine 退出，
// 避免 fx OnStop 后续阶段（如 db 关闭）与清理循环产生资源竞争。
// 当 ctx 超时时返回 ctx.Err()，调用方应自行决定是否重试或放弃。
func (c *WorkspaceCleaner) Stop(ctx context.Context) error {
	close(c.stopCh)
	if c.scanTicker != nil {
		c.scanTicker.Stop()
	}
	select {
	case <-c.doneCh:
		logx.Infof("workspace cleaner stopped")
		return nil
	case <-ctx.Done():
		logx.Warnf("workspace cleaner stop timed out: %v", ctx.Err())
		return ctx.Err()
	}
}
