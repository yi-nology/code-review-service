package ocr

import (
	"sync"
)

// Client 是 open-code-review 能力的 full 入口（阶段 A 终极版）。
//
// 与 Registry 的区别：
//   - Registry 是薄壳（每个 getter 转发到底层 manager）
//   - Client 持有 Config 缓存 + Settings 接口 + 全局生命周期
//
// 阶段 A+8 完成时：
//   - 升级 Registry 为 Client（向后兼容 Registry 类型）
//   - Client 内部持有 Runner/Factory/Config/refreshMu
//   - 提供 Refresh(ctx) 方法异步更新所有依赖
//
// 阶段 A 当前：Client 已存在最简结构（持有 Config + 缓存锁），
// 但 getter 仍委托给 Registry 模式。
type Client struct {
	mu      sync.RWMutex
	cfg     Config
	refresh bool // 占位：阶段 A+8 用于驱动 refresh
}

// NewClient 构造 Client（最简形式，阶段 A 占位）。
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// Cfg 返回当前 Config。
func (c *Client) Cfg() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

// Refresh 替换 Config（阶段 A 占位，阶段 A+8 接入 settings 拉取 + 异步）。
func (c *Client) Refresh(cfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
	c.refresh = true
}

// WasRefreshed 报告 Client 是否被 Refresh 过（测试用）。
func (c *Client) WasRefreshed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.refresh
}

// 编译期断言：Client 满足基本的 Getter 接口。
var _ interface {
	Cfg() Config
} = (*Client)(nil)
