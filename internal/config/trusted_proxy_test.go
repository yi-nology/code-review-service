package config

import "testing"

// TestGetTrustedProxySet 覆盖可信代理清单的解析口径。
//
// 关键约定：非法条目必须报错而不是静默跳过——跳过会让「配了却没生效」与
// 「没配」在运行期完全同形（上游 pkg#104 的教训）。
func TestGetTrustedProxySet(t *testing.T) {
	t.Run("empty list yields nil set", func(t *testing.T) {
		c := &Config{}
		set, err := c.GetTrustedProxySet()
		if err != nil {
			t.Fatalf("空清单不应报错: %v", err)
		}
		if set != nil {
			t.Fatal("空清单应返回 nil set（= 不信任任何代理头）")
		}
	})

	t.Run("valid cidr and bare ip parse", func(t *testing.T) {
		c := &Config{Proxy: ProxyConfig{TrustedProxies: []string{"172.16.0.0/12", "10.0.0.5"}}}
		set, err := c.GetTrustedProxySet()
		if err != nil {
			t.Fatalf("合法清单不应报错: %v", err)
		}
		if set == nil {
			t.Fatal("合法清单应返回非 nil set")
		}
	})

	t.Run("invalid entry errors instead of being skipped", func(t *testing.T) {
		c := &Config{Proxy: ProxyConfig{TrustedProxies: []string{"not-an-ip"}}}
		if _, err := c.GetTrustedProxySet(); err == nil {
			t.Fatal("非法条目应报错（静默跳过 = 配置悄悄失效）")
		}
	})
}

// TestValidateRejectsInvalidTrustedProxies 钉住启动即失败：可信代理清单写错
// 时服务必须拒绝启动，而不是带着未生效的配置跑起来。
func TestValidateRejectsInvalidTrustedProxies(t *testing.T) {
	c := &Config{
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://x/y"},
		Auth:     AuthConfig{Token: "real-token"},
		Proxy:    ProxyConfig{TrustedProxies: []string{"999.999.999.999/8"}},
	}
	if err := c.Validate("prod"); err == nil {
		t.Fatal("非法 trusted_proxies 应让 Validate 失败")
	}
}
