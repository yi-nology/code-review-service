#!/usr/bin/env python3
"""生成某音色的全部配音（神经网络 TTS）。

用法：
    python3 tools/gen_voice.py v_xiaoxiao zh-CN-XiaoxiaoNeural
    python3 tools/gen_voice.py v_yunxi    zh-CN-YunxiNeural
    python3 tools/gen_voice.py v_yunyang  zh-CN-YunyangNeural

产物落在 out/tts/<tag>/*.mp3，渲染时用 PROMO_VOICE=<tag> 选中。

依赖：edge-tts（装在 venv 里；PEP 668 禁止装进系统 python）：
    python3 -m venv /tmp/ttsenv && /tmp/ttsenv/bin/pip install edge-tts
可用 EDGE_TTS 环境变量指定 edge-tts 可执行文件路径。
"""
from __future__ import annotations

import os
import pathlib
import re
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "tools"))

from shots import VIDEOS  # noqa: E402

EDGE = os.environ.get("EDGE_TTS", "/tmp/ttsenv/bin/edge-tts")


def split_sentences(text: str) -> list[str]:
    parts = re.findall(r"[^。！？!?；;]*[。！？!?；;]?", text)
    return [x.strip() for x in parts if x.strip()]


def main() -> None:
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)
    tag, voice = sys.argv[1], sys.argv[2]
    if not pathlib.Path(EDGE).exists():
        print(f"找不到 edge-tts：{EDGE}\n"
              f"  python3 -m venv /tmp/ttsenv && /tmp/ttsenv/bin/pip install edge-tts")
        sys.exit(1)

    out = ROOT / "out" / "tts" / tag
    out.mkdir(parents=True, exist_ok=True)

    jobs = []
    for key in ("A", "B"):
        for shot in VIDEOS[key]:
            for i, s in enumerate(split_sentences(shot["narration"])):
                jobs.append((f"{shot['id']}_{i:02d}", s))

    for name, text in jobs:
        dst = out / f"{name}.mp3"
        # 一律重新生成，不吃缓存：口播文案改了但文件名没变时，
        # 跳过会留下旧音频 —— 与「转换缓存没按音色隔离」是同一类静默错误。
        # edge-tts 偶发网络抖动，重试几次
        last = None
        for attempt in range(4):
            try:
                subprocess.run([EDGE, "--voice", voice, "--text", text,
                                "--write-media", str(dst)], check=True)
                last = None
                break
            except subprocess.CalledProcessError as e:
                last = e
                print(f"   重试 {attempt + 1}/4: {name}")
                time.sleep(2 * (attempt + 1))
        if last:
            raise last
        print("  ", name)
    print(f"{tag} 共 {len(jobs)} 句 -> {out}")


if __name__ == "__main__":
    main()
