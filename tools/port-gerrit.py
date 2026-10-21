#!/usr/bin/env python3
"""把内部树的 Gerrit 支持移植到公共树。

背景
----
内部树（跑生产的那棵）已经解决 Gerrit 的注册问题：Gerrit 不在 go-git-platform
SDK 的 backend 列表里，走独立的 go-gerrit 客户端。公共树落后于内部树。

移植方式
--------
**整文件复制 + 模块路径改写**。前提是这些文件在两棵树之间**只差 Gerrit 部分**
——已用 `diff -u` 逐个核对过，差异全部落在 Gerrit 相关代码上。

不走「整树同步」是因为公共树有大量并行的在途工作（开源发布准备：
`docs/archive/`、`sanitize_proxy.py`、`CODE_OF_CONDUCT.md`、README 改写……），
整体覆盖会把那些一起冲掉。

用法：python3 tools/port-gerrit.py [--check]
"""
from __future__ import annotations

import pathlib
import subprocess
import sys

INT = pathlib.Path("/Users/zhangyi/my_project/review-service")
PUB = pathlib.Path("/Users/zhangyi/my_project/code-review-service")

SRC = "git.enjoye.top/enjoydream/code-review-service"
DST = "github.com/yi-nology/code-review-service"

# 逐个核对过：这些文件在两棵树之间只差 Gerrit 相关代码
FILES = [
    "internal/domain/platform/platform.go",
    "internal/service/gerrit/holder.go",          # 公共树没有，新建
    "internal/service/gerrit/client.go",
    "internal/service/provider/service.go",
    "internal/modules/gerrit.go",
    "internal/modules/provider.go",
    "internal/modules/writeback.go",
    "internal/service/writeback/gerrit_adapter.go",
]

FRONTEND = [
    "frontend/src/utils/url-parser.ts",
    "frontend/src/views/GitPlatformSettings.vue",
]


def port(check_only: bool = False) -> int:
    changed = []
    for rel in FILES:
        src_p = INT / rel
        dst_p = PUB / rel
        if not src_p.exists():
            print(f"✗ 内部树缺文件：{rel}")
            return 1
        body = src_p.read_text(encoding="utf-8").replace(SRC, DST)
        if dst_p.exists() and dst_p.read_text(encoding="utf-8") == body:
            print(f"· {rel}  已一致")
            continue
        changed.append(rel)
        if not check_only:
            dst_p.parent.mkdir(parents=True, exist_ok=True)
            dst_p.write_text(body, encoding="utf-8")
            print(f"✓ {rel}")

    for rel in FRONTEND:
        src_p = INT / rel
        dst_p = PUB / rel
        body = src_p.read_text(encoding="utf-8").replace(SRC, DST)
        if dst_p.read_text(encoding="utf-8") == body:
            print(f"· {rel}  已一致")
            continue
        changed.append(rel)
        if not check_only:
            dst_p.write_text(body, encoding="utf-8")
            print(f"✓ {rel}")

    if check_only:
        print(f"\n待移植 {len(changed)} 个文件")
        return 0

    # 编译验证
    print("\n=== go build ===")
    r = subprocess.run(["go", "build", "./..."], cwd=PUB, capture_output=True, text=True)
    if r.returncode != 0:
        print(r.stdout + r.stderr)
        return 1
    print("编译通过 ✓")
    return 0


if __name__ == "__main__":
    sys.exit(port("--check" in sys.argv))
