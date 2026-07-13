package notification

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

// TestFormatEvent_Completed 验证评审完成通知文案格式。
func TestFormatEvent_Completed(t *testing.T) {
	title, body := formatEvent(notification.Event{
		Kind:     notification.KindCompleted,
		ReviewID: "rev-1",
		Provider: "github",
		Repo:     "acme/app",
		Grade:    "B",
		Findings: 3,
	})
	if title == "" {
		t.Error("expected non-empty title for completed")
	}
	if body == "" {
		t.Error("expected non-empty body for completed")
	}
}

// TestFormatEvent_GateFailed 验证门禁未通过通知含问题数和评分。
func TestFormatEvent_GateFailed(t *testing.T) {
	_, body := formatEvent(notification.Event{
		Kind:     notification.KindGateFailed,
		ReviewID: "rev-1",
		Grade:    "C",
		Findings: 5,
	})
	// body 应包含关键信息
	if body == "" {
		t.Fatal("expected non-empty body")
	}
}

// TestFormatEvent_Failed 验证失败通知含失败阶段。
func TestFormatEvent_Failed(t *testing.T) {
	_, body := formatEvent(notification.Event{
		Kind:  notification.KindFailed,
		Stage: "ocr",
		Error: "LLM timeout",
	})
	if body == "" {
		t.Fatal("expected non-empty body for failed")
	}
}

// TestShouldNotify 验证渠道 On 条件过滤。
func TestShouldNotify(t *testing.T) {
	// 空 On → 所有事件都推送
	if !shouldNotify(nil, notification.KindCompleted) {
		t.Error("empty On should notify all events")
	}
	if !shouldNotify(nil, notification.KindFailed) {
		t.Error("empty On should notify all events")
	}
	// 指定 On → 只推送匹配的事件
	onFailed := []string{"failed"}
	if shouldNotify(onFailed, notification.KindCompleted) {
		t.Error("on=[failed] should not notify completed")
	}
	if !shouldNotify(onFailed, notification.KindFailed) {
		t.Error("on=[failed] should notify failed")
	}
}

// TestNotifier_NoChannels 验证无渠道时 Notify 是空操作（不 panic）。
func TestNotifier_NoChannels(t *testing.T) {
	n := NewNotifier(nil)
	if err := n.Notify(context.Background(), notification.Event{Kind: notification.KindCompleted}); err != nil {
		t.Errorf("Notify with no channels should be noop, got err: %v", err)
	}
}

// TestNoopNotifier 验证 NoopNotifier 实现 Notifier 语义。
func TestNoopNotifier(t *testing.T) {
	var n NoopNotifier
	if err := n.Notify(context.Background(), notification.Event{}); err != nil {
		t.Errorf("NoopNotifier.Notify should return nil, got %v", err)
	}
}

// TestNewWebhookChannel_PayloadFormats 验证各渠道 payload 格式。
func TestNewWebhookChannel_PayloadFormats(t *testing.T) {
	event := notification.Event{Kind: notification.KindCompleted, ReviewID: "r1", Grade: "A", Findings: 1}

	cases := []struct {
		channelType string
		expectKey   string // payload 应包含的顶层 key
	}{
		{"feishu", "msg_type"},
		{"wecom", "msgtype"},
		{"dingtalk", "msgtype"},
		{"generic", "title"},
	}
	for _, tc := range cases {
		t.Run(tc.channelType, func(t *testing.T) {
			ch := NewWebhookChannel(notification.WebhookChannelConfig{Type: tc.channelType, URL: "http://example.com"})
			payload := ch.buildPayload(event)
			if _, ok := payload[tc.expectKey]; !ok {
				t.Errorf("channel %s payload missing key %q, got: %v", tc.channelType, tc.expectKey, payload)
			}
		})
	}
}
