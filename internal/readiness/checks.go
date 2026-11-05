// Package readiness 提供就绪/健康探针所需的检查器。
//
// 这些检查器被 modules（server.go / preflight.go / probe.go）用于注册 /health、
// /healthz、/readyz 路由，以及在启动时做依赖预检与周期探活。
// 从 internal/modules 抽离为独立包，使 modules 仅负责 fx 装配、不承载检查逻辑。
package readiness

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	ocrcore "github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/hertz/health"
	"github.com/oekit/logx"
	"gorm.io/gorm"
)

// DBCheck 检查数据库连通性。返回 health.HealthChecker（exported 函数类型），
// 以便用 hertz.NamedCheckers 变参展开，绕开 ekit health.namedChecker 跨包不可命名的问题。
func DBCheck(db *gorm.DB) health.HealthChecker {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
}

// OCRBinaryCheck 检查 OCR 二进制是否存在于 PATH。
// 轻量（exec.LookPath 不 fork），用于 /health 就绪探针。
// 二进制缺失会让所有评审失败，因此作为就绪条件。
func OCRBinaryCheck(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) health.HealthChecker {
	return func(ctx context.Context) error {
		s, err := repos.OCRSettings.Get(ctx)
		if err != nil || s == nil || !s.Enabled {
			return nil
		}
		// 配置读取失败（如首次启动尚未配置）时用默认 binary 检查，
		// 不应阻断就绪探针——配置问题由 preflight 与日志反映。
		// 但此前错误被静默吞掉，DB 配置层故障时仍报 ready，首条评审才暴露。改为记录告警。
		settings := ocrcore.NewSettingsAdapter(repos.OCRSettings)
		data, settingsErr := settings.Get(ctx)
		if settingsErr != nil {
			logx.Warnf("readiness: read OCR settings failed, using default binary: %v", settingsErr)
		}
		cfg, resolveErr := resolver.Resolve(ctx, ocrcore.OCROverrides{})
		if resolveErr != nil {
			logx.Warnf("readiness: resolve OCR config failed, using default binary: %v", resolveErr)
		}
		_ = data
		binary := cfg.Binary
		if binary == "" {
			binary = "ocr"
		}
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("ocr binary %q not found in PATH: %w", binary, err)
		}
		return nil
	}
}

// LLMEndpointCheck 对配置的 LLM endpoint 做轻量 TCP 拨号（不带认证、不发请求体），
// 仅验证 DNS 解析与端口可达。OCR 未启用或未配置 URL 时跳过。
func LLMEndpointCheck(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) health.HealthChecker {
	return func(ctx context.Context) error {
		// 总预算 4s：双探各 1.5s + 退避 0.3s + 余量（抖动豁免需容纳两次完整拨号）。
		ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()

		s, err := repos.OCRSettings.Get(ctx)
		if err != nil || s == nil || !s.Enabled {
			return nil
		}
		settings := ocrcore.NewSettingsAdapter(repos.OCRSettings)
		data, settingsErr := settings.Get(ctx)
		if settingsErr != nil {
			logx.Warnf("readiness: read OCR settings failed for LLM probe: %v", settingsErr)
		}
		cfg := resolver.ResolveFromSettings(data, ocrcore.OCROverrides{})
		if cfg.URL == "" {
			return nil
		}
		u, err := url.Parse(cfg.URL)
		if err != nil {
			return fmt.Errorf("parse LLM url %q: %w", cfg.URL, err)
		}
		host := u.Hostname()
		port := u.Port()
		if host == "" {
			return fmt.Errorf("LLM url %q missing host", cfg.URL)
		}
		if port == "" {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		addr := net.JoinHostPort(host, port)
		return dialWithFlapTolerance(ctx, addr, dialLLM)
	}
}

// dialFunc 拨号形态（可注入测试替身）。
type dialFunc func(ctx context.Context, addr string) (net.Conn, error)

// flapRetryDelay 首探失败到复探的退避（测试可覆盖）。
var flapRetryDelay = 300 * time.Millisecond

// dialLLM 生产拨号（单次 1.5s 超时）。
func dialLLM(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 1500 * time.Millisecond}
	return d.DialContext(ctx, "tcp", addr)
}

// dialWithFlapTolerance 拨号 + DNS 抖动豁免（#诊断页误报修复，可测抽出）：
// 瞬时 DNS 抖动（215 实测 30 分钟偶发 2 次 lookup 超时，TCP+TLS 与评审全正常）
// 会让单探偶发失败——单次失败不代表端点不可达，短退避后再探一次；两次全失败
// 才报错。探针是趋势信号，不被单次抖动打红。
func dialWithFlapTolerance(ctx context.Context, addr string, dial dialFunc) error {
	conn, err := dial(ctx, addr)
	if err == nil {
		_ = conn.Close()
		return nil
	}
	select {
	case <-ctx.Done():
		// ctx 中断≠不可达（探针被取消 vs 端点故障是两种处置）——按 ctx.Err() 透出。
		return fmt.Errorf("LLM endpoint %s probe interrupted: %w", addr, ctx.Err())
	case <-time.After(flapRetryDelay):
	}
	conn2, err2 := dial(ctx, addr)
	if err2 != nil {
		return fmt.Errorf("LLM endpoint %s unreachable（连探 2 次）: %w", addr, err2)
	}
	_ = conn2.Close()
	return nil
}

// usabilityCacheTTL 可用性探测结果缓存（防 /health 轮询放大 LLM 调用）。
const usabilityCacheTTL = 5 * time.Minute

// usabilityState 一次可用性探测的缓存结果。
type usabilityState struct {
	mu      sync.Mutex
	at      time.Time
	lastErr error
}

// llmUsability 可用性探测结果缓存（进程级单例：checker 可能被多个路由共享）。
var llmUsability usabilityState

// probe 取可用性结果（命中缓存 TTL 直接回；否则真探测）。
func (u *usabilityState) probe(fn func() error) error {
	u.mu.Lock()
	if time.Since(u.at) < usabilityCacheTTL {
		err := u.lastErr
		u.mu.Unlock()
		return err
	}
	u.mu.Unlock()
	err := fn()
	u.mu.Lock()
	u.at, u.lastErr = time.Now(), err
	u.mu.Unlock()
	return err
}

// LLMAvailabilityCheck LLM 可用性探测（v3.179.1，「可达≠可用」盲区修复）：
// 1-token chat completion——200=可用；429=配额尽/限流（诊断页给出语义红灯，
// 而非 TCP 探针绿灯假象——215 配额耗尽事件实证）；401/403=凭证面；其他=
// 端点故障。结果缓存 5 分钟防 /health 轮询放大调用；anthropic 协议暂跳过
// （openai 兼容形态是主用面，协议分派探测留后续）。
func LLMAvailabilityCheck(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) health.HealthChecker {
	return func(ctx context.Context) error {
		return llmUsability.probe(func() error {
			s, err := repos.OCRSettings.Get(ctx)
			if err != nil || s == nil || !s.Enabled {
				return nil
			}
			settings := ocrcore.NewSettingsAdapter(repos.OCRSettings)
			data, _ := settings.Get(ctx)
			cfg := resolver.ResolveFromSettings(data, ocrcore.OCROverrides{})
			if cfg.URL == "" || cfg.UseAnthropic {
				return nil
			}
			return probeLLMUsability(ctx, cfg.URL, cfg.AuthToken, cfg.AuthHeader, cfg.Model)
		})
	}
}

// probeLLMUsability 1-token 可用性请求（openai 兼容 chat/completions）。
func probeLLMUsability(ctx context.Context, baseURL, token, authHeader, model string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	h := authHeader
	if h == "" {
		h = "Authorization"
	}
	// Authorization 头统一拼 Bearer scheme；自定义头（如 X-API-Key）保持原样。
	// 此前 auth_header="Authorization"（显式配置）会走自定义分支裸发 token，
	// 网关 401（生产实测：显式配置 Authorization 头的网关会拒绝裸 token）。
	if strings.EqualFold(h, "Authorization") {
		req.Header.Set(h, "Bearer "+token)
	} else {
		req.Header.Set(h, token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("LLM 可用性探测网络失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("LLM 不可用（429）：配额尽或限流——check token-plan 用量/充配额")
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("LLM 不可用（%d）：凭证失效——check OCR settings 的 auth", resp.StatusCode)
	default:
		return fmt.Errorf("LLM 不可用（HTTP %d）", resp.StatusCode)
	}
}
