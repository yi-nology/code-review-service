package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/oekit/logx"
)

// WebhookChannel 通过 HTTP POST 推送通知到外部地址（飞书/企业微信/钉钉/通用）。
// 按 channel 配置的 Type 构造对应平台的 payload 格式。
type WebhookChannel struct {
	cfg    notification.WebhookChannelConfig
	client *http.Client
}

func NewWebhookChannel(cfg notification.WebhookChannelConfig) *WebhookChannel {
	return &WebhookChannel{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *WebhookChannel) Name() string { return c.cfg.Type }

func (c *WebhookChannel) Send(ctx context.Context, event notification.Event) error {
	if !shouldNotify(c.cfg.On, event.Kind) {
		return nil
	}
	payload := c.buildPayload(event)
	return c.post(ctx, payload)
}

// buildPayload 按渠道类型构造请求体。
func (c *WebhookChannel) buildPayload(event notification.Event) map[string]any {
	title, text := formatEvent(event)
	switch strings.ToLower(c.cfg.Type) {
	case "feishu", "lark":
		return map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": title + "\n" + text},
		}
	case "wecom", "wechat":
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": title + "\n" + text},
		}
	case "dingtalk", "dingding":
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": title + "\n" + text},
		}
	default: // generic：标准 JSON
		return map[string]any{
			"title": title,
			"text":  text,
			"event": event,
		}
	}
}

// post 发送 webhook，带签名（飞书/钉钉）和简单重试（5xx/429）。
// 重试模式参考 internal/ocr/llm.go doHTTP。
func (c *WebhookChannel) post(ctx context.Context, payload map[string]any) error {
	const maxAttempts = 2
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Second + time.Duration(rand.Intn(500))*time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal webhook payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		c.sign(req, body)

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook call failed: %w", err)
			continue
		}
		// 读取并关闭 body
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		lastErr = fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
		if !retryable {
			return lastErr
		}
	}
	if lastErr != nil {
		logx.Warnf("[notification] webhook %s push failed after retries: %v", c.cfg.Type, lastErr)
	}
	return lastErr
}

// sign 按渠道类型添加签名头/参数。飞书用 timestamp+sign，钉钉用 timestamp+sign 查询参数。
// 无 secret 时不签名。
func (c *WebhookChannel) sign(req *http.Request, body []byte) {
	if c.cfg.Secret == "" {
		return
	}
	switch strings.ToLower(c.cfg.Type) {
	case "feishu", "lark":
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		stringToSign := timestamp + "\n" + c.cfg.Secret
		h := hmac.New(sha256.New, []byte(stringToSign))
		h.Write(body)
		sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Lark-Signature", sign)
		req.Header.Set("X-Lark-Request-Timestamp", timestamp)
	case "dingtalk", "dingding":
		timestamp := time.Now().UnixMilli()
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, c.cfg.Secret)
		h := hmac.New(sha256.New, []byte(c.cfg.Secret))
		h.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
		q := req.URL.Query()
		q.Set("timestamp", fmt.Sprintf("%d", timestamp))
		q.Set("sign", sign)
		req.URL.RawQuery = q.Encode()
	}
}

// hexSHA256 工具（通用 generic 渠道可选 HMAC 校验预留）。
func hexSHA256(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
