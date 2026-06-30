// Package redact 提供日志 / 错误信息中的敏感数据脱敏工具。
//
// 背景：runner.ExitError 把子进程 stderr 整体写入错误信息，stderr 中常包含
// LLM 提供方回显的 Authorization / Bearer / api_key 等敏感字段。如果把这些
// 信息无脑写到 logx.Infof / logx.Errorf，会泄露到日志收集系统。
//
// 用法：
//
//	if err != nil {
//	    return redact.Errorf(err, "ocr review failed: %s", path)
//	}
//
// 或者把字符串先过 redact.String(s) 再传给 logger。
package redact

import (
	"fmt"
	"regexp"
	"strings"
)

// pattern 覆盖常见的密钥形态。匹配时做大小写不敏感。
var patterns = []*regexp.Regexp{
	// Authorization: Bearer xxx / Basic xxx
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer|basic|token)\s+[^\s"',;}{]+`),
	// 独立出现的 Bearer / Token 关键字 + 至少 8 位的值
	regexp.MustCompile(`(?i)\b(bearer|token)\s+[A-Za-z0-9._\-+/=]{8,}`),
	// api_key=xxx / apiKey:xxx / "api_key": "xxx"
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|access[_-]?key|secret[_-]?key|private[_-]?key)\s*["']?\s*[:=]\s*["']?[^\s"',;}{][^,}\s"';]*`),
	// GitHub / GitLab / Gitea personal access token 形态
	regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_\-]{20,})`),
	// X-Api-Key / X-Auth-Token headers
	regexp.MustCompile(`(?i)(x-api-key|x-auth-token|x-token)\s*[:=]\s*[^\s"',;}{]+`),
	// OpenAI sk-xxx
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
}

// String 返回 s 的脱敏版本：把匹配到的敏感字段替换为 ***。
func String(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, p := range patterns {
		out = p.ReplaceAllStringFunc(out, redactMatch)
	}
	return out
}

// redactedError 包装 error，在 Error() 输出时自动脱敏，同时保留 Unwrap 链。
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string {
	if e.err == nil {
		return String(e.msg)
	}
	return String(e.msg) + ": " + String(e.err.Error())
}

func (e *redactedError) Unwrap() error { return e.err }

// Errorf 包装 fmt.Errorf 同时对底层 err 做脱敏。
// 保留 errors.Is / errors.As 链路（Unwrap 实现）。
func Errorf(err error, format string, args ...any) error {
	return &redactedError{
		msg: fmt.Sprintf(format, args...),
		err: err,
	}
}

func redactMatch(s string) string {
	// 保留前缀（如 "Authorization: "），但替换掉 value
	idx := indexKeySep(s)
	if idx < 0 || idx >= len(s)-1 {
		return "***"
	}
	prefix := s[:idx+1]
	rest := strings.TrimSpace(s[idx+1:])
	// 如果包含 Bearer/Token/Basic 等，保留关键字后替换
	for _, kw := range []string{"Bearer", "bearer", "Basic", "basic", "Token", "token"} {
		if strings.HasPrefix(rest, kw+" ") || rest == kw {
			return prefix + kw + " ***"
		}
	}
	return prefix + "***"
}

// indexKeySep 查找 "key + : / =" 分隔符位置。
func indexKeySep(s string) int {
	for i, c := range s {
		if c == ':' || c == '=' {
			return i
		}
	}
	return -1
}
