package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/oekit/concurrency/retry"
	retryhttpx "github.com/oekit/concurrency/retry/httpx"
	"github.com/oekit/logx"
	"github.com/yi-nology/code-review-service/internal/domain/notification"
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

// webhookRetryAttempts 是 webhook 推送的总尝试次数（含首次）。
//
// 注意 retry.Config 的**零值是只跑一次**——与本仓旧的手搓重试（零值 2 次）
// 相反，所以这里必须显式写。
const webhookRetryAttempts = 2

// post 发送 webhook，带签名（飞书/钉钉）和简单重试（5xx/429）。
//
// 重试判据走 oekit/concurrency/retry：退避、抖动、ctx 取消、可重试判定
// 只有一份实现（此前是本仓手搓的 internal/httpx，与 oekit 的语义漂移过一次）。
//
// 「哪些该重试」由 retry/httpx.Classifier() 判定：传输层失败与 429/5xx 重试，
// 其余 4xx 与 ctx 取消不重试。响应体**不参与**判定——webhook 渠道的语义是
// 「任意 2xx 即成功」，不强制 JSON 响应体（这一点是内部/httpx 注释里点名的
// 保留理由，因此也不能用 pkg/httpjson 的严格 JSON 语义）。
func (c *WebhookChannel) post(ctx context.Context, payload map[string]any) error {
	err := retry.Do(ctx, retry.Config{
		Attempts:  webhookRetryAttempts,
		Delay:     time.Second,
		Jitter:    500 * time.Millisecond,
		Retryable: retryhttpx.Classifier(),
	}, func(attempt int) error {
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
			return fmt.Errorf("webhook call failed: %w", err)
		}
		// 读取并关闭 body
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// 带上状态码，让 Classifier 判定 429/5xx 重试、其余 4xx 不重试。
		return &retryhttpx.HTTPError{
			Status: resp.StatusCode,
			Err:    fmt.Errorf("webhook returned HTTP %d", resp.StatusCode),
		}
	})
	if err != nil {
		logx.Warnf("[notification] webhook %s push failed after retries: %v", c.cfg.Type, err)
	}
	return err
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
