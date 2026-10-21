package gerrit

import (
	"sync"

	"github.com/oekit/logx"
)

// ClientHolder 持有当前生效的 Gerrit 客户端，支持运行期热更新。
//
// 为什么是 holder 而不是每次现建：writeback 适配器在装配期（fx Provide 阶段）
// 就被构造出来。若那时把 *Client 直接捕获进去，之后在 web console 改平台
// 配置（换 token / 换地址 / 禁用）都不会生效——只能重启进程。持有 getter
// 后，适配器每次取用时拿当前客户端，热更新才真正落地。
//
// holder 放在 service 层而不是装配层：biz/handler 只允许依赖 service / domain
// （不能 import internal/modules，否则与装配层成环），平台设置页保存配置后
// 要能触发热更新，路径必须走得到这里。
type ClientHolder struct {
	mu     sync.RWMutex
	client *Client
}

// Get 返回当前客户端；未配置时返回 nil（调用方需判空）。
func (h *ClientHolder) Get() *Client {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.client
}

// Set 替换当前客户端（传 nil 表示清除配置）。
func (h *ClientHolder) Set(cl *Client) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.client = cl
	h.mu.Unlock()
}

// Reload 用新配置重建客户端并替换当前客户端。
//
// baseURL 或 token 为空时视为「清除配置」（平台被禁用 / 删除），
// 此时返回 nil 而不是报错——清空不是失败。
func (h *ClientHolder) Reload(baseURL, token string, skipTLS bool) error {
	if h == nil {
		return nil
	}
	if baseURL == "" || token == "" {
		h.Set(nil)
		return nil
	}
	cl, err := NewClient(baseURL, token, skipTLS)
	if err != nil {
		return err
	}
	h.Set(cl)
	logx.Info("gerrit client reloaded", "base_url", baseURL, "skip_tls", skipTLS)
	return nil
}

// Clear 清除当前客户端（平台被禁用 / 删除时调用）。
func (h *ClientHolder) Clear() {
	h.Set(nil)
}
