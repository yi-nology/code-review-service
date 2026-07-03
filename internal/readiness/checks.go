// Package readiness 提供就绪/健康探针所需的检查器。
//
// 这些检查器被 modules（server.go / preflight.go / probe.go）用于注册 /health、
// /healthz、/readyz 路由，以及在启动时做依赖预检与周期探活。
// 从 internal/modules 抽离为独立包，使 modules 仅负责 fx 装配、不承载检查逻辑。
package readiness

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"time"

	ocrcore "github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/hertz"
	"github.com/oekit/logx"
	"gorm.io/gorm"
)

// DBCheck 检查数据库连通性。返回 hertz.HealthChecker（exported 函数类型），
// 以便用 hertz.NamedCheckers 变参展开，绕开 ekit health.namedChecker 跨包不可命名的问题。
func DBCheck(db *gorm.DB) hertz.HealthChecker {
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
func OCRBinaryCheck(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) hertz.HealthChecker {
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
func LLMEndpointCheck(repos *repository.RepoContainer, resolver *ocrcore.ConfigResolver) hertz.HealthChecker {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
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
		d := net.Dialer{Timeout: 1500 * time.Millisecond}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("LLM endpoint %s unreachable: %w", addr, err)
		}
		_ = conn.Close()
		return nil
	}
}

// RegistryOCRHealthChecks 一次性构造 OCR + LLM 两个 readiness check。
// B6 新增：用 *ocrcore.Registry 代替 (repos, resolver) 两个参数。
//
// 阶段 A 完整迁移后，OCRBinaryCheck / LLMEndpointCheck 这两个老函数可删除，
// 所有调用点改为 RegistryOCRHealthChecks(reg)。
func RegistryOCRHealthChecks(reg *ocrcore.Registry, resolver *ocrcore.ConfigResolver) []hertz.HealthChecker {
	return []hertz.HealthChecker{
		OCRBinaryCheck(reg.Repos(), resolver),
		LLMEndpointCheck(reg.Repos(), resolver),
	}
}
