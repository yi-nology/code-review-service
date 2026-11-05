# Kubernetes 部署

本目录提供 review-service 全家桶（API + MCP server + Web 控制台 + PostgreSQL）的
Kubernetes manifests，与 `deploy/Dockerfile.frontend` / 仓库根 `Dockerfile` 的
容器约定一致。生产验证过的部署形态，注意事项已内嵌为注释。

## 文件清单

| 文件 | 内容 |
| --- | --- |
| `00-postgres.yaml` | PostgreSQL StatefulSet + headless Service + 凭据 Secret（占位） |
| `10-review-service.yaml` | review-service ConfigMap + Deployment + Service + 凭据 Secret（占位） |
| `20-mcp-server.yaml` | mcp-server Deployment + Service |
| `30-review-frontend.yaml` | review-frontend（Web 控制台）Deployment + Service |

## 部署步骤

```sh
# 1. 把两个 Secret 里的 change-me 占位改成真实值
#    （REVIEW_TOKEN = 控制台/API 访问令牌；DATABASE_URL 指向 00-postgres.yaml 里的库）
$EDITOR deploy/k8s/00-postgres.yaml deploy/k8s/10-review-service.yaml

# 2. 按序应用（namespace 任意；本例用 review-service）
kubectl apply -f deploy/k8s/

# 3. 验证
kubectl -n review-service get pods
kubectl -n review-service port-forward svc/review-frontend 8091:80
# 打开 http://localhost:8091 ，用 REVIEW_TOKEN 登录控制台配置平台/OCR/策略
```

## 注意事项（生产实测）

- **探针用 `tcpSocket`，不要用 `/healthz`**：`/healthz` 包含 `llm-usable` 检查（LLM
  配额尽/凭证失效时 503），作为 liveness 会把好端在的 pod 无限重启。TCP 探针只保证
  进程存活；LLM 红灯通过 `/health` API 观测即可（控制台健康页有语义化展示）。
- **前端容器自带 `/api` 反代**（`deploy/nginx.conf` 静态 `proxy_pass` 指向
  `review-service:8080`），Service 名必须是 `review-service`（本目录已按此命名）。
- **Secret 不进 git**：两个 Secret 是清单内嵌的（change-me 占位）。若走 GitOps，
  建议改为 `kubectl create secret` 外部注入并从清单中删除。
- **数据面**：PostgreSQL 用 StatefulSet + 默认 StorageClass。生产建议换成集群已有
  的托管 PG（只需改 `DATABASE_URL` 并删除 `00-postgres.yaml`）。
- **镜像架构**：GHCR 镜像是 amd64+arm64 双架构；离线（无外网）集群用
  `docker save | k3s ctr images import`（containerd 命名空间必须 `-n k8s.io`）导入。
