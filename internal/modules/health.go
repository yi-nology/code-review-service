package modules

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/hertz/health"
	"gorm.io/gorm"
)

// dbCheck 检查数据库连通性。返回 health.HealthChecker（exported 函数类型），
// 以便用 health.NamedCheckers 变参展开，绕开 ekit health.namedChecker 跨包不可命名的问题。
func dbCheck(db *gorm.DB) health.HealthChecker {
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

// ocrBinaryCheck 检查 OCR 二进制是否存在于 PATH。
// 轻量（exec.LookPath 不 fork），用于 /health 就绪探针。
// 二进制缺失会让所有评审失败，因此作为就绪条件。
func ocrBinaryCheck(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) health.HealthChecker {
	return func(ctx context.Context) error {
		s, err := repos.OCRSettings.Get(ctx)
		if err != nil || s == nil || !s.Enabled {
			return nil
		}
		// 配置读取失败（如首次启动尚未配置）时用默认 binary 检查，
		// 不应阻断就绪探针——配置问题由 preflight 与日志反映。
		cfg, _ := resolver.Resolve(review.OCROverrides{})
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

// llmEndpointCheck 对配置的 LLM endpoint 做轻量 TCP 拨号（不带认证、不发请求体），
// 仅验证 DNS 解析与端口可达。OCR 未启用或未配置 URL 时跳过。
// 这是本次 DNS 事故的关键缺口：之前 LLM 挂掉 /health 仍返回 200。
func llmEndpointCheck(repos *repository.RepoContainer, resolver *ocr.ConfigResolver) health.HealthChecker {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		s, err := repos.OCRSettings.Get(ctx)
		if err != nil || s == nil || !s.Enabled {
			return nil
		}
		cfg := resolver.ResolveFromSettings(s, review.OCROverrides{})
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
