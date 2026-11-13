package middleware

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/hertz/clientip"
	mw "github.com/oekit/middleware"
	"github.com/oekit/pkg/convert"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware 返回限流中间件。
// 委托给 ekit 的 mw.RateLimitMiddleware：默认按 ClientIP 分桶（KeyedLimiter，每 key
// 独立令牌桶 + TTL 清理），非全局共享预算；需要全局预算时显式传 WithRateLimitKeyFunc。
//
// trusted 为空时沿用上游默认口径（按 TCP 对端地址分桶）。反代拓扑下上游默认
// 取的是 nginx 的地址 ⇒ 所有客户端共用一个桶，等于全局预算；注入可信代理集合后
// 改为按真实客户端 IP 分桶（可信代理集合非 nil 时才注入，见上）。
func RateLimitMiddleware(rps int, burst int, trusted *convert.TrustedProxySet) app.HandlerFunc {
	opts := []mw.RateLimitOption{mw.WithRateLimit(rate.Limit(rps), burst)}
	if trusted != nil {
		// clientip.Resolver：清单在构造期解析一次，请求期零 CIDR 解析
		// （hertz v0.11.0 #174 的预构造形态）。
		resolver := clientip.NewFromSet(trusted)
		opts = append(opts, mw.WithRateLimitKeyFunc(func(c *app.RequestContext) string {
			return resolver.GetClientIP(c)
		}))
	}
	return mw.RateLimitMiddleware(opts...)
}

// AccessLogClientIPFunc 返回访问日志的客户端 IP 解析函数。
//
// 上游 oekit/middleware v0.14.1 起 accesslog 的来源 IP 改为 fail-closed：
// 只取 TCP 对端地址、不采信任何代理头（hertz 的 c.ClientIP() 默认信任任意来源的
// XFF，直连方可伪造 ⇒ 审计链的来源 IP 可被改写）。直连部署下这是对的；但本服务
// 在生产跑在 nginx 之后（deploy/nginx.conf 反代 /api 并写 X-Forwarded-For），
// 不注入解析缝的话，升级后访问日志里的来源 IP 会全部退化成 nginx 容器地址。
//
// trusted 为 nil（未配置可信代理）时返回 nil —— 上游按 fail-closed 的对端地址口径
// 记录，语义与升级前一致（升级前 c.ClientIP() 采信 XFF，但那时也是无认证采信）。
func AccessLogClientIPFunc(trusted *convert.TrustedProxySet) func(remoteAddr, xff string, trustedProxies []string) string {
	if trusted == nil {
		return nil
	}
	return func(remoteAddr, xff string, _ []string) string {
		return convert.ResolveClientIPWithSet(remoteAddr, xff, trusted)
	}
}
