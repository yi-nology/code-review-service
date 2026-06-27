package cleanup

import (
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
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start 启动清理：先立即清理一次，再按 interval 定期扫描。
func (c *WorkspaceCleaner) Start() {
	logx.Infof("starting workspace cleaner, baseDir=%s, pattern=%s, maxAge=%v, interval=%v",
		c.baseDir, c.pattern, c.maxAge, c.interval)

	c.cleanup()

	c.scanTicker = time.NewTicker(c.interval)
	go func() {
		for {
			select {
			case <-c.scanTicker.C:
				c.cleanup()
			case <-c.stopCh:
				return
			}
		}
	}()
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

func (c *WorkspaceCleaner) Stop() {
	close(c.stopCh)
	if c.scanTicker != nil {
		c.scanTicker.Stop()
	}
	logx.Infof("workspace cleaner stopped")
}
