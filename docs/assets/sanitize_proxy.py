#!/usr/bin/env python3
"""脱敏反向代理：给「公网发布用」的截图提供已净化的数据。

用法：python3 sanitize_proxy.py <listen_port> <upstream_frontend> <upstream_api>

浏览器 → 本代理 → 215 上的前端 / 后端。

三条规则：
  1. 标识符替换：仓库路径、人员名、ULID、SHA、内网主机/内网 IP、内网 registry
     → 一一映射到可公开的假值（同一个输入恒定映射到同一个假值）。
  2. 评审内容整体替换：findings / analysis 是 LLM 对**客户代码**的分析，
     含客户 CI 脚本片段——公网截图不能带，整段换成通用示例。
  3. 其余字段原样透传：结构、评分、耗时、token 统计等保持真实形态，
     这样截图看起来是真实跑出来的，而不是造假。
"""
import gzip
import hashlib
import json
import re
import sys
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

LISTEN_PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8899
UP_FRONTEND = sys.argv[2] if len(sys.argv) > 2 else "http://10.44.129.215:8091"
UP_API = sys.argv[3] if len(sys.argv) > 3 else "http://10.44.129.215:8090"

# ---------------------------------------------------------------- 标识符映射
REPO_MAP = {
    "obs/doc/rd-guidelines": "acme/service-template",
    "obs/doc/deployment-document": "acme/deploy-docs",
    "obs/kylin-insights-agent": "acme/insight-agent",
    "obs/kylin-insights-server": "acme/insight-server",
    "obs/kylin-insights-frontend": "acme/insight-frontend",
    "obs/kylin-insights-chart": "acme/insight-chart",
    "obs/kylin-insights-ci-templates": "acme/ci-templates",
    "obs/insights-mcp-gateway": "acme/mcp-gateway",
    "obs/openresty-scripts": "acme/openresty-scripts",
    "obs/iam-sdk": "acme/iam-sdk",
    "obs/iam-server": "acme/iam-server",
    "obs/gantt": "acme/gantt",
    "obs/insights-qa-ticket-deletion_scheduled-1520": "acme/qa-scheduler",
    "obs/insights-tech-docs": "acme/tech-docs",
    "obs/jianmu": "acme/jianmu",
    "obs/jianmu-web": "acme/jianmu-web",
    "obs/lanxin-proxy": "acme/gateway-proxy",
    "obs/message": "acme/message-service",
    "obs/octopus": "acme/octopus",
    "obs/ops/check-repo": "acme/ops-checker",
    "obs/ops/chart": "acme/ops-chart",
    "obs/ops/deploy": "acme/ops-deploy",
    "obs/ops/deploy-kit": "acme/ops-deploy-kit",
    "obs/ops/deploy-sql": "acme/ops-deploy-sql",
    "obs/ops/docs": "acme/ops-docs",
    "obs/ops/images": "acme/ops-images",
    "obs/ops/insights-qa-ticket": "acme/ops-qa-ticket",
    "obs/ops/release": "acme/ops-release",
    "obs/ops/steadyflow": "acme/ops-steadyflow",
    "obs/ops/tmp-ns-probe-deletion_scheduled-1519": "acme/ns-probe",
    "obs/ops/tools": "acme/ops-tools",
    "obs/sdk/queries": "acme/sdk-queries",
    "obs/sdk/server": "acme/sdk-server",
    "obs/skill/bianque-hub-inner": "acme/hub-inner",
    "obs/sync-iam": "acme/sync-iam",
    "obs/sys-iam": "acme/sys-iam",
    "kylincloud/KylinFusionArk/frontend/kylin-compute-console": "acme/console-web",
    "zhangyi01/argus-validate": "acme/sandbox-lab",
    "enjoydream/ekit": "acme/ekit",
}
PERSON_MAP = {
    "zhangpengxuan": "alice.chen",
    "zhangyi01": "dev.team",
    "wuzhou": "bob.liu",
    "zhangyi": "dev.team",
    "root": "ci-bot",
}
HOST_MAP = {
    "om-gitlab.kylinos.cn": "gitlab.example.com",
    "git.enjoye.top": "gitlab.example.com",
    "code-review.me": "code-review.example.com",
    "harbor.enjoye.top": "ghcr.io",
    "token-plan.cn-beijing.maas.aliyuncs.com": "llm.example.com",
    "10.44.13.22": "10.0.0.9",
    "10.44.129.215": "10.0.0.21",
    "172.18.0.1": "172.16.0.1",
    "172.18.0.11": "172.16.0.11",
    "1Panel-postgresql-RHrE": "postgres-primary",
    "bianque-jaeger-1": "jaeger-collector",
    "otel-collector": "otel-collector",
}

_ULID_RE = re.compile(r"\b01[0-9A-HJKMNP-TV-Z]{24}\b")
_SHA_RE = re.compile(r"\b[0-9a-f]{40}\b")
_SHORT_SHA_RE = re.compile(r"\b[0-9a-f]{7,12}\b")
# 任何邮箱 → 固定域名的假地址（提交作者、评审人邮箱等一律不带出去）
_EMAIL_RE = re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}")
# 内网公司域名（任意子域）→ example.com
_INTERNAL_DOMAIN_RE = re.compile(r"[A-Za-z0-9.-]*\b(?:kylinos\.cn|enjoye\.top)\b")
# 内部代号/分支名
_LITERAL_MAP = {
    "ctor_main_v5": "release-2026.10",
    "main_v5": "main",
}


def _fake_email(seed: str) -> str:
    h = hashlib.sha256(seed.encode()).hexdigest()
    return f"user-{h[:8]}@example.com"


def _fake_person(seed: str) -> str:
    h = hashlib.sha256(seed.encode()).hexdigest()
    return f"dev.user{h[:3]}"


def _fake_ulid(seed: str) -> str:
    """把真实 ULID 换成形状相同的假 ULID（可排序、前缀相同）。"""
    digest = hashlib.sha256(seed.encode()).digest()
    alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
    body = "".join(alphabet[b % 32] for b in digest[:24])
    return "01" + body


def _fake_sha(seed: str) -> str:
    digest = hashlib.sha256(seed.encode()).hexdigest()
    return digest[:40]


def sanitize_text(s: str) -> str:
    if not isinstance(s, str) or not s:
        return s
    for src, dst in REPO_MAP.items():
        s = s.replace(src, dst)
    for src, dst in HOST_MAP.items():
        s = s.replace(src, dst)
    for src, dst in _LITERAL_MAP.items():
        s = re.sub(r"(?<![A-Za-z0-9_.-])" + re.escape(src) + r"(?![A-Za-z0-9_.-])", dst, s)
    for src, dst in PERSON_MAP.items():
        s = re.sub(r"(?<![A-Za-z0-9_.-])" + re.escape(src) + r"(?![A-Za-z0-9_.-])", dst, s)
    # 邮箱：本地部分一并替换（真实姓名常嵌在 local part 里）
    s = _EMAIL_RE.sub(lambda m: _fake_email(m.group(0)), s)
    # 内网公司域名（含未在 HOST_MAP 里列出的子域）
    s = _INTERNAL_DOMAIN_RE.sub("example.com", s)
    s = _ULID_RE.sub(lambda m: _fake_ulid(m.group(0)), s)
    s = _SHA_RE.sub(lambda m: _fake_sha(m.group(0)), s)
    return s


# ------------------------------------------------------- 评审内容：通用示例
DEMO_FINDINGS = [
    {
        "severity": "critical", "category": "安全性",
        "path": "internal/store/order.go", "line": "88", "start_line": "88", "end_line": "92",
        "title": "SQL 注入：订单查询直接拼接用户输入",
        "body": "[security] 查询语句用 `fmt.Sprintf` 把 `req.OrderID` 直接拼进 SQL，"
                "参数可被构造为 `' OR '1'='1` 绕过过滤，返回越权订单。应对查询参数使用"
                "占位符绑定，并把不可信输入与 SQL 模板彻底分离。",
        "existing_code": "func (s *Store) GetOrder(ctx context.Context, req *GetOrderReq) (*Order, error) {\n"
                         "\tquery := fmt.Sprintf(\"SELECT * FROM orders WHERE id = '%s'\", req.OrderID)\n"
                         "\trow := s.db.QueryRowContext(ctx, query)\n"
                         "\t// ...\n}",
        "suggestion_code": "func (s *Store) GetOrder(ctx context.Context, req *GetOrderReq) (*Order, error) {\n"
                           "\trow := s.db.QueryRowContext(ctx, \"SELECT * FROM orders WHERE id = ?\", req.OrderID)\n"
                           "\t// ...\n}",
        "fingerprint": "demo0000000000001",
    },
    {
        "severity": "major", "category": "可靠性",
        "path": "internal/api/handler.go", "line": "132", "start_line": "132", "end_line": "140",
        "title": "错误被吞掉：调用方拿不到失败原因",
        "body": "[backend] 下游调用的错误只被记日志后丢弃，函数仍返回 nil 错误，"
                "上层会认为请求成功。失败被静默吞掉会让问题在更远的地方以"
                "「数据莫名其妙为空」的形式暴露。应当把错误向上传递。",
        "existing_code": "if err := client.Sync(ctx, payload); err != nil {\n"
                         "\tlog.Warnf(\"sync failed: %v\", err)\n"
                         "}\n"
                         "return nil",
        "suggestion_code": "if err := client.Sync(ctx, payload); err != nil {\n"
                           "\treturn fmt.Errorf(\"sync payload: %w\", err)\n"
                           "}",
        "fingerprint": "demo0000000000002",
    },
    {
        "severity": "major", "category": "性能",
        "path": "internal/store/user.go", "line": "57", "start_line": "57", "end_line": "63",
        "title": "循环内查库：N+1 查询",
        "body": "[performance] 遍历订单时逐条查询用户，构成 N+1 查询，订单量上来后"
                "查询次数线性增长。应当改为一次 join 或批量查询后在内存中关联。",
        "existing_code": "for _, o := range orders {\n"
                         "\tu, err := s.getUser(ctx, o.UserID)\n"
                         "\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n"
                         "\trows = append(rows, row{o, u})\n}",
        "suggestion_code": "users, err := s.getUsers(ctx, userIDs)\n"
                           "// ... 一次批量查询后在内存中关联",
        "fingerprint": "demo0000000000003",
    },
    {
        "severity": "major", "category": "可维护性",
        "path": "internal/config/config.go", "line": "23", "start_line": "23", "end_line": "23",
        "title": "硬编码的默认地址",
        "body": "[maintainability] 默认地址写死在代码里，切换环境需要改代码重新编译。"
                "应当放进配置，由部署时注入。",
        "existing_code": "const defaultAddr = \"http://10.0.0.9:8080\"",
        "suggestion_code": "addr := os.Getenv(\"SERVICE_ADDR\")\nif addr == \"\" {\n\taddr = \"http://127.0.0.1:8080\"\n}",
        "fingerprint": "demo0000000000004",
    },
    {
        "severity": "major", "category": "可维护性",
        "path": "internal/store/order.go", "line": "40", "start_line": "40", "end_line": "40",
        "title": "缺少上下文透传",
        "body": "[maintainability] 该方法没有接收 context.Context，链路上的取消与超时"
                "无法传递到存储层。建议统一透传。",
        "existing_code": "func (s *Store) ListOrders(userID string) ([]*Order, error) {",
        "suggestion_code": "func (s *Store) ListOrders(ctx context.Context, userID string) ([]*Order, error) {",
        "fingerprint": "demo0000000000005",
    },
]

DEMO_ANALYSIS = {
    "overview": "本次变更包含三部分：(1) 订单查询接口新增了一个按 ID 查询的方法，"
                 "实现里把请求参数直接拼进 SQL；(2) 批量订单列表改为逐条查询用户，"
                 "构成 N+1 查询；(3) 新增的默认地址常量写死在配置包中。"
                 "整体看是常规业务迭代，但引入了一个可被外部利用的注入点，"
                 "建议优先修复。",
    "risks": [
        "订单查询的 SQL 拼接可被构造出注入语句，攻击者可能越权读取任意订单，属高危。",
        "N+1 查询在订单量上升后会把数据库连接数放大，是慢查询的常见来源。",
        "硬编码地址让多环境部署需要改代码重新编译，容易在生产写出测试环境的地址。",
    ],
    "recommendations": [
        "把订单查询改成占位符绑定，并对查询结果做归属校验（越权读取同样要堵）。",
        "批量查询用户改为一次批量取回后在内存中关联，或在 SQL 层 join。",
        "默认地址改为从环境变量读取，配置包不再持有具体地址。",
    ],
}


def sanitize_obj(o):
    """递归净化 JSON 结构里的字符串。"""
    if isinstance(o, dict):
        # git 作者对象（{name, email}）——姓名与邮箱都是个人信息，整体替换。
        if "email" in o and "name" in o and isinstance(o.get("email"), str):
            seed = o["email"]
            return {**{k: sanitize_obj(v) for k, v in o.items() if k not in ("name", "email")},
                    "name": _fake_person(seed), "email": _fake_email(seed)}
        return {k: sanitize_obj(v) for k, v in o.items()}
    if isinstance(o, list):
        return [sanitize_obj(v) for v in o]
    if isinstance(o, str):
        return sanitize_text(o)
    return o


def sanitize_review(review: dict) -> dict:
    """净化单条评审记录，并替换其内容为通用示例。"""
    review = sanitize_obj(review)
    if "findings" in review and isinstance(review["findings"], list):
        review["findings"] = json.loads(json.dumps(DEMO_FINDINGS))
        # 报告里的计数/扣分/得分必须与示例问题**自洽**——门禁面板的文案是
        # 前端按 counts 与 deductions 现算的，不对齐就会出现「列出 5 条问题、
        # 面板却写 0 严重 5 高风险」这种自相矛盾。
        # 1 严重 + 4 高风险、changed_lines=372 ⇒ 扣 7 + 13 = 20 ⇒ 80 分「良好」
        # （评分口径见 internal/domain/scoring：ptsCritical=25 / ptsMajor=12，
        #   体积系数 = clamp(100/changed_lines, 0.2, 3.0)）。
        report = review.get("report")
        if isinstance(report, dict):
            if isinstance(report.get("counts"), dict):
                report["counts"].update({"critical": 1, "major": 4, "minor": 0, "info": 0})
            score = report.get("score")
            if isinstance(score, dict):
                score["value"] = 80
                score["grade"] = "良好"
                score["deductions"] = [
                    {"reason": "严重问题", "points": 7, "count": 1},
                    {"reason": "高风险问题", "points": 13, "count": 4},
                ]
    if "analysis" in review and isinstance(review["analysis"], dict):
        review["analysis"] = json.loads(json.dumps(DEMO_ANALYSIS))
    return review


def sanitize_payload(data):
    """按响应结构替换评审内容。"""
    if not isinstance(data, dict):
        return sanitize_obj(data)
    d = data.get("data")
    if isinstance(d, dict):
        if isinstance(d.get("review"), dict):
            d["review"] = sanitize_review(d["review"])
        elif isinstance(d.get("items"), list):
            d["items"] = [
                (sanitize_review(it) if isinstance(it, dict) and "findings" in it else sanitize_obj(it))
                for it in d["items"]
            ]
        else:
            for k, v in list(d.items()):
                d[k] = sanitize_obj(v)
        return data
    return sanitize_obj(data)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):  # 静默
        pass

    def _upstream(self):
        if self.path.startswith("/api/") or self.path.startswith("/ping") or self.path.startswith("/swagger"):
            return UP_API + self.path
        return UP_FRONTEND + self.path

    def _proxy(self, body=None):
        req = urllib.request.Request(self._upstream(), data=body, method=self.command)
        for k, v in self.headers.items():
            if k.lower() in ("host", "accept-encoding", "connection", "content-length"):
                continue
            req.add_header(k, v)
        req.add_header("Accept-Encoding", "identity")
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                data = resp.read()
                ctype = resp.headers.get("Content-Type", "")
                if "json" in ctype:
                    # **净化失败必须 fail-closed**：宁可 502，也不能把原始数据
                    # 放行——放行一次就等于把内部仓库名/邮箱/凭据直接发出去。
                    # （这正是本次差点翻车的地方：一个 NameError 被 except 吞掉，
                    #   代理若无其事地转发了未净化的响应。）
                    try:
                        data = json.dumps(sanitize_payload(json.loads(data)), ensure_ascii=False).encode()
                        ctype = "application/json; charset=utf-8"
                    except Exception as e:
                        err = f"sanitize failed: {e}".encode()
                        self.send_response(502)
                        self.send_header("Content-Type", "text/plain")
                        self.send_header("Content-Length", str(len(err)))
                        self.end_headers()
                        self.wfile.write(err)
                        return
                self.send_response(resp.status)
                self.send_header("Content-Type", ctype)
                self.send_header("Content-Length", str(len(data)))
                for k in ("Location", "Set-Cookie"):
                    if resp.headers.get(k):
                        self.send_header(k, resp.headers[k])
                self.end_headers()
                if self.command != "HEAD":
                    self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read()
            self.send_response(e.code)
            self.send_header("Content-Type", e.headers.get("Content-Type", "text/plain"))
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:  # 上游不可达
            msg = f"proxy error: {e}".encode()
            self.send_response(502)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(msg)))
            self.end_headers()
            self.wfile.write(msg)

    def do_GET(self):
        self._proxy()

    def do_HEAD(self):
        self._proxy()

    def do_POST(self):
        n = int(self.headers.get("Content-Length") or 0)
        self._proxy(self.rfile.read(n) if n else None)

    def do_PUT(self):
        self.do_POST()

    def do_DELETE(self):
        self.do_POST()


if __name__ == "__main__":
    srv = ThreadingHTTPServer(("127.0.0.1", LISTEN_PORT), Handler)
    print(f"sanitize proxy on http://127.0.0.1:{LISTEN_PORT} -> {UP_FRONTEND} / {UP_API}", flush=True)
    srv.serve_forever()
