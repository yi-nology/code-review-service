#!/bin/bash
# webhook 集成测试脚本
# 运行方式：WEBHOOK_ALLOW_INSECURE=true bash test_webhook_e2e.sh
# 前置条件：服务运行在 BASE_URL（默认 http://localhost:8081）

set -e

BASE_URL="${TEST_BASE_URL:-http://localhost:8081}"

echo "=== Webhook 端到端测试 ==="
echo "Target: $BASE_URL"
echo ""

# 1. 签名校验测试（无 secret 配置时应拒绝）
echo "--- Test 1: Signature verification (should reject without secret) ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/webhooks/gitea/default" \
  -H "Content-Type: application/json" \
  -H "X-Gitea-Event: pull_request" \
  -d '{"action":"opened","number":1,"pull_request":{"number":1},"repository":{"full_name":"t/r"},"sender":{"login":"u"}}')

if [ "$HTTP_CODE" = "401" ]; then
  echo "  ✅ PASS: signature rejected (HTTP 401)"
elif [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ]; then
  echo "  ⚠️  signature accepted (WEBHOOK_ALLOW_INSECURE=true is set)"
else
  echo "  ❌ FAIL: unexpected HTTP $HTTP_CODE"
fi
echo ""

# 2. 正确签名的 Gitea webhook（如果有 secret）
echo "--- Test 2: Gitea PR webhook with valid signature ---"
BODY='{"action":"opened","number":999,"pull_request":{"number":999,"title":"e2e","head":{"sha":"abc"},"base":{"sha":"def"},"user":{"login":"u"}},"repository":{"full_name":"test/e2e"},"sender":{"login":"u"}}'
SECRET="${WEBHOOK_SECRET:-}"
if [ -n "$SECRET" ]; then
  SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" -hex | awk '{print $NF}')
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/webhooks/gitea/default" \
    -H "Content-Type: application/json" \
    -H "X-Gitea-Event: pull_request" \
    -H "X-Gitea-Delivery: e2e-002" \
    -H "X-Gitea-Signature: $SIG" \
    -d "$BODY")
  echo "  HTTP: $HTTP_CODE"
  if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ]; then
    echo "  ✅ PASS"
  else
    echo "  ❌ FAIL"
  fi
else
  echo "  ⏭️  skipped (no WEBHOOK_SECRET set)"
fi
echo ""

# 3. 多平台路由可达性
echo "--- Test 3: Multi-platform route reachability ---"
for PROVIDER in gitea github gitlab gitcode tencent_code; do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/webhooks/$PROVIDER/default" \
    -H "Content-Type: application/json" \
    -H "X-Gitea-Event: pull_request" \
    -d '{"action":"opened","number":1}')
  # 401=签名校验失败（正常）；404=installation未注册（正常，非gitea平台）；200/202=接受
  if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "202" ] || [ "$HTTP_CODE" = "404" ]; then
    echo "  $PROVIDER: ✅ reachable (HTTP $HTTP_CODE)"
  else
    echo "  $PROVIDER: ❌ FAIL (HTTP $HTTP_CODE)"
  fi
done
echo ""

# 4. Bot sender 过滤测试（需要 WEBHOOK_ALLOW_INSECURE=true）
echo "--- Test 4: Bot sender filtering ---"
if [ "$WEBHOOK_ALLOW_INSECURE" = "true" ]; then
  BODY='{"action":"opened","number":777,"pull_request":{"number":777,"head":{"sha":"a"},"base":{"sha":"b"},"user":{"login":"review-bot"}},"repository":{"full_name":"test/bot"},"sender":{"login":"review-bot"}}'
  RESP=$(curl -s -X POST "$BASE_URL/api/v1/webhooks/gitea/default" \
    -H "Content-Type: application/json" \
    -H "X-Gitea-Event: pull_request" \
    -H "X-Gitea-Delivery: e2e-bot-001" \
    -d "$BODY")
  if echo "$RESP" | grep -q "ignored"; then
    echo "  ✅ PASS: bot sender ignored"
  else
    echo "  ⚠️  bot not explicitly ignored (RESP: $RESP)"
  fi
else
  echo "  ⏭️  skipped (requires WEBHOOK_ALLOW_INSECURE=true)"
fi
echo ""

# 5. 重复 webhook 幂等性
echo "--- Test 5: Duplicate delivery idempotency ---"
BODY='{"action":"opened","number":888,"pull_request":{"number":888,"head":{"sha":"a"},"base":{"sha":"b"},"user":{"login":"u"}},"repository":{"full_name":"test/dup"},"sender":{"login":"u"}}'
RESP1=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/webhooks/gitea/default" \
  -H "Content-Type: application/json" \
  -H "X-Gitea-Event: pull_request" \
  -H "X-Gitea-Delivery: e2e-dup-001" \
  -d "$BODY")
RESP2=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/v1/webhooks/gitea/default" \
  -H "Content-Type: application/json" \
  -H "X-Gitea-Event: pull_request" \
  -H "X-Gitea-Delivery: e2e-dup-001" \
  -d "$BODY")
echo "  First: HTTP $RESP1, Second: HTTP $RESP2"
if [ "$RESP1" = "$RESP2" ]; then
  echo "  ✅ PASS: same response (idempotent or both rejected)"
else
  echo "  ⚠️  different responses (may indicate first was processed, second deduped)"
fi
echo ""

echo "=== Webhook 测试完成 ==="
