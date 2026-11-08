// redirect_guard.go argus 出站重定向同源钳制（GO-2026-6911 自持缓解）。
//
// 背景：mcp-go（GO-2026-6911，Fixed in: N/A）的 StreamableHTTP transport
// 默认构造不带 CheckRedirect 的 http.Client，30x 的 Location 头不校验即跟随，
// 且 Authorization 等静态头挂在请求上——恶意/被攻陷的 argus 端点可借重定向
// 把凭证外带到任意主机（SSRF / 凭证泄露）。上游修复 PR（mark3labs/mcp-go
// #1190 / #1192）截至 2026-11-09 仍未合并，故在本仓调用面自持缓解：
// 出站重定向只放行与配置的 argus base_url 同源（scheme+主机）的目标，
// 跨源 Location 一律拒绝。REST 通道（c.client）同源钳制属同类加固。
package argusclient

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// maxSameOriginRedirects 同源重定向跳数上限。CheckRedirect 非空时 stdlib
// 默认的 10 跳上限失效，需自设；argus 正常部署不需要多跳，取保守值。
const maxSameOriginRedirects = 3

// ErrRedirectBlocked 重定向被同源钳制拒绝（GO-2026-6911 自持缓解）。
// 用 errors.Is 判定；包裹的错误里带拒绝原因（跨源 / 超跳数 / base_url 不可解析）。
var ErrRedirectBlocked = errors.New("argus 重定向被拒绝（GO-2026-6911 自持缓解）")

// newRedirectGuardedClient 给 client 装上同源重定向钳制并原样返回：
// Location 解析后与 baseURL 同源才放行（同源含相对跳转，如 LB 的路径改写），
// 其余拒绝并返回 ErrRedirectBlocked。client 的既有属性（如 Timeout）保留——
// MCP SSE 长流依赖无整体超时的 client，调用方按通道各自传入。
func newRedirectGuardedClient(client *http.Client, baseURL string) *http.Client {
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		base, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("%w: base_url %q 不可解析: %v", ErrRedirectBlocked, baseURL, err)
		}
		if !sameOrigin(req.URL, base) {
			from := req.URL
			if len(via) > 0 {
				from = via[0].URL
			}
			return fmt.Errorf("%w: %s -> %s，仅允许同源重定向", ErrRedirectBlocked, from, req.URL)
		}
		if len(via) > maxSameOriginRedirects {
			return fmt.Errorf("%w: 同源重定向超过 %d 跳", ErrRedirectBlocked, maxSameOriginRedirects)
		}
		return nil
	}
	return client
}

// sameOrigin 两 URL 是否同源（scheme + 主机，默认端口折叠：http://x:80 与
// http://x 同源）。大小写不敏感。
func sameOrigin(a, b *url.URL) bool {
	return originKey(a) == originKey(b)
}

// originKey 归一化 origin 键："scheme://host[:port]"。
func originKey(u *url.URL) string {
	if u == nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		return scheme + "://" + host + ":" + port
	}
	return scheme + "://" + host
}
