#!/bin/bash

# GitHub Webhook 测试脚本（带签名验证）
BASE_URL="http://localhost:8080"
PROVIDER="github"
INSTALLATION="default"
WEBHOOK_SECRET="test-webhook-secret-123"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 计算 HMAC SHA256 签名
compute_signature() {
    local payload="$1"
    echo -n "$payload" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | sed 's/^.* //'
}

echo -e "${YELLOW}=== GitHub Webhook 测试 ===${NC}"

# 测试 Pull Request Hook
test_pull_request() {
    echo -e "\n${YELLOW}测试 Pull Request Hook...${NC}"
    
    PAYLOAD='{
        "action": "opened",
        "number": 1,
        "pull_request": {
            "number": 1,
            "title": "Test Pull Request",
            "state": "open",
            "draft": false,
            "head": {
                "sha": "abc123def456",
                "ref": "feature/test"
            },
            "base": {
                "sha": "base123sha456",
                "ref": "main"
            },
            "user": {
                "login": "zy84338719",
                "avatar_url": "https://avatars.githubusercontent.com/u/test"
            }
        },
        "repository": {
            "id": 12345,
            "full_name": "zy84338719/test-repo",
            "name": "test-repo",
            "owner": {
                "login": "zy84338719"
            }
        },
        "sender": {
            "login": "zy84338719"
        }
    }'
    
    SIGNATURE=$(compute_signature "$PAYLOAD")
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/api/v1/webhooks/${PROVIDER}/${INSTALLATION}" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: pull_request" \
        -H "X-GitHub-Delivery: test-uuid-$(date +%s)" \
        -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
        -d "$PAYLOAD")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "202" ]; then
        echo -e "${GREEN}✓ Pull Request Hook 测试成功 (HTTP $HTTP_CODE)${NC}"
        echo "Response: $BODY"
    else
        echo -e "${RED}✗ Pull Request Hook 测试失败 (HTTP $HTTP_CODE)${NC}"
        echo "Response: $BODY"
    fi
}

# 测试 Push Hook
test_push() {
    echo -e "\n${YELLOW}测试 Push Hook...${NC}"
    
    PAYLOAD='{
        "ref": "refs/heads/main",
        "before": "0000000000000000000000000000000000000000",
        "after": "abc123def456789",
        "repository": {
            "id": 12345,
            "full_name": "zy84338719/test-repo",
            "name": "test-repo"
        },
        "sender": {
            "login": "zy84338719"
        },
        "commits": [
            {
                "id": "abc123def456789",
                "message": "test commit"
            }
        ]
    }'
    
    SIGNATURE=$(compute_signature "$PAYLOAD")
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/api/v1/webhooks/${PROVIDER}/${INSTALLATION}" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: push" \
        -H "X-GitHub-Delivery: test-push-uuid-$(date +%s)" \
        -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
        -d "$PAYLOAD")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "202" ]; then
        echo -e "${GREEN}✓ Push Hook 测试成功 (HTTP $HTTP_CODE)${NC}"
        echo "Response: $BODY"
    else
        echo -e "${RED}✗ Push Hook 测试失败 (HTTP $HTTP_CODE)${NC}"
        echo "Response: $BODY"
    fi
}

# 主逻辑
case "${1:-all}" in
    pr|pull_request)
        test_pull_request
        ;;
    push)
        test_push
        ;;
    all)
        test_pull_request
        test_push
        ;;
    *)
        echo "用法: $0 [pr|push|all]"
        exit 1
        ;;
esac

echo -e "\n${YELLOW}=== 测试完成 ===${NC}"
