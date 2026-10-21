#!/bin/bash

# GitCode Webhook 测试脚本
# 使用方法: ./test_gitcode_webhook.sh [merge_request|push]

BASE_URL="http://localhost:8080"
PROVIDER="gitcode"
INSTALLATION="default"  # 根据实际情况修改

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== GitCode Webhook 测试 ===${NC}"

# 测试 Merge Request Hook
test_merge_request() {
    echo -e "\n${YELLOW}测试 Merge Request Hook...${NC}"
    
    PAYLOAD='{
        "object_kind": "merge_request",
        "object_attributes": {
            "iid": 1,
            "title": "Test Merge Request",
            "action": "open",
            "state": "opened",
            "last_commit": {
                "id": "abc123def456"
            },
            "diff_refs": {
                "base_sha": "base123sha456",
                "head_sha": "abc123def456"
            }
        },
        "project": {
            "id": 12345,
            "path_with_namespace": "example-user/test-api-1781165929863739000",
            "name": "test-api"
        },
        "user": {
            "username": "example-user",
            "name": "Test User",
            "avatar_url": "https://gitcode.com/avatar/test.png"
        }
    }'
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/api/v1/webhooks/${PROVIDER}/${INSTALLATION}" \
        -H "Content-Type: application/json" \
        -H "X-GitLab-Event: Merge Request Hook" \
        -H "X-Gitlab-Event-UUID: test-uuid-$(date +%s)" \
        -d "$PAYLOAD")
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "202" ]; then
        echo -e "${GREEN}✓ Merge Request Hook 测试成功 (HTTP $HTTP_CODE)${NC}"
        echo "Response: $BODY"
    else
        echo -e "${RED}✗ Merge Request Hook 测试失败 (HTTP $HTTP_CODE)${NC}"
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
            "full_name": "example-user/test-api-1781165929863739000",
            "name": "test-api"
        },
        "user_username": "example-user",
        "commits": [
            {
                "id": "abc123def456789",
                "message": "test commit",
                "author": {
                    "name": "Test User",
                    "email": "test@example.com"
                }
            }
        ]
    }'
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/api/v1/webhooks/${PROVIDER}/${INSTALLATION}" \
        -H "Content-Type: application/json" \
        -H "X-GitLab-Event: Push Hook" \
        -H "X-Gitlab-Event-UUID: test-push-uuid-$(date +%s)" \
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
    merge_request|mr)
        test_merge_request
        ;;
    push)
        test_push
        ;;
    all)
        test_merge_request
        test_push
        ;;
    *)
        echo "用法: $0 [merge_request|push|all]"
        exit 1
        ;;
esac

echo -e "\n${YELLOW}=== 测试完成 ===${NC}"
