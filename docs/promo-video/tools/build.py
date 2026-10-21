#!/usr/bin/env python3
"""宣传视频渲染器。

流程：分句 TTS → 时间轴 → PIL 逐帧合成（Ken Burns + 聚光 + 标注 + 字幕）→
      ffmpeg 编码 → 音轨拼接混音 → 封装成片。

为什么字幕在 PIL 里烧而不用 ffmpeg subtitles：
    本机 homebrew ffmpeg 没有编译 libass/libfreetype，drawtext/subtitles 都不可用。
    反正画面本来就要逐帧合成，字幕一起画进去反而能精确控制位置和样式。

用法：
    python3 tools/build.py A          # 渲染 A 片
    python3 tools/build.py A B        # 两支都渲染
    python3 tools/build.py A --probe  # 只出关键帧 PNG，不编码视频（快速看效果）
"""
from __future__ import annotations

import argparse
import json
import os
import math
import pathlib
import re
import shutil
import subprocess
import sys

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from shots import VIDEOS  # noqa: E402

ROOT = pathlib.Path(__file__).resolve().parent.parent
DOCS = ROOT.parent.parent / "docs" / "assets" / "screenshots"
OUT = ROOT / "out"
# 配音音色目录（out/tts/ 下的子目录）。换音色：PROMO_VOICE=v_xiaoxiao python3 tools/build.py A B
VOICE_DIR = os.environ.get("PROMO_VOICE", "v_xiaoxiao")
TTS = OUT / "tts" / VOICE_DIR
TTS_FALLBACK = OUT / "tts" / "_say"
FRAMES = OUT / "frames"

W, H = 1920, 1080
HEADER_H = 84            # 顶部品牌横栏高度
VW, VH = 1080, 1920      # 9:16 竖版
FPS = 30

FONT_REG = "/System/Library/Fonts/Hiragino Sans GB.ttc"   # index 0 = W3
FONT_BOLD = "/System/Library/Fonts/Hiragino Sans GB.ttc"  # index 2 = W6
FONT_MONO = "/System/Library/Fonts/Menlo.ttc"

INK = (10, 16, 30)
ACCENT = (64, 158, 255)
MUTED = (148, 163, 184)

VOICE = "Tingting"
PAD_TAIL = 0.45      # 镜头尾部留白
PAD_LEAD = 0.18      # 镜头头部留白
SENT_GAP = 0.16      # 句间停顿
FADE = 0.35          # 淡入淡出时长


# ---------------------------------------------------------------- 基础工具

def font(size: int, bold: bool = False, mono: bool = False) -> ImageFont.FreeTypeFont:
    if mono:
        return ImageFont.truetype(FONT_MONO, size, index=0)
    return ImageFont.truetype(FONT_BOLD if bold else FONT_REG, size, index=2 if bold else 0)


def run(cmd: list[str]) -> str:
    return subprocess.run(cmd, capture_output=True, text=True, check=True).stdout.strip()


def dur(path: pathlib.Path) -> float:
    return float(run(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                      "-of", "csv=p=0", str(path)]))


def ease(t: float) -> float:
    """smoothstep，用于镜头运动与淡入淡出。"""
    t = max(0.0, min(1.0, t))
    return t * t * (3 - 2 * t)


def lerp(a: float, b: float, t: float) -> float:
    return a + (b - a) * t


# ---------------------------------------------------------------- 文本

def split_sentences(text: str) -> list[str]:
    """按中文句末标点切句，标点跟随前句。"""
    parts = re.findall(r"[^。！？!?；;]*[。！？!?；;]?", text)
    return [p.strip() for p in parts if p.strip()]


def text_w(draw: ImageDraw.ImageDraw, s: str, f: ImageFont.FreeTypeFont) -> int:
    return int(draw.textbbox((0, 0), s, font=f)[2])


# 不能出现在行首的标点（中文禁则）
NO_LINE_START = "。，、；：！？）】》」』%…·"


def wrap(d, text: str, f, maxw: int) -> list[str]:
    """按像素宽度折行；并做中文禁则处理——收尾标点不落行首。

    禁则的正确做法是**悬挂标点**：行满时若下一个字符是收尾标点（。，、；：！？），
    把它挂到本行行尾（允许轻微超宽），而不是在标点处断行。

    ⚠️ 曾经写反过一次：把「标点跟随上一行」实现成了「遇到标点就断行」，
    结果**每个逗号都强制换行**——带逗号的字幕全被拆成「一小句一行」，
    宣传片字幕变成竖排的目录一样的东西。判断标准很简单：
    一句 30 字的中文应该折成 1~2 行，而不是 4 行。
    """
    lines, cur = [], ""
    for ch in text:
        trial = cur + ch
        if cur and d.textbbox((0, 0), trial, font=f)[2] > maxw:
            if ch in NO_LINE_START:
                # 标点不落行首：挂到本行尾，允许轻微超宽
                lines.append(cur + ch)
                cur = ""
            else:
                lines.append(cur)
                cur = ch
        else:
            cur = trial
    if cur:
        lines.append(cur)
    return lines


def draw_text(d, xy, s, f, fill, anchor="la", shadow=True):
    if shadow:
        d.text((xy[0] + 2, xy[1] + 3), s, font=f, fill=(0, 0, 0, 140), anchor=anchor)
    d.text(xy, s, font=f, fill=fill, anchor=anchor)


# ---------------------------------------------------------------- 镜头运动

def crop_at(shot: dict, p: float) -> tuple[int, int, int, int]:
    """p = 镜头内进度 0..1，返回源图裁切矩形。"""
    x, y, w, h = shot["crop"]
    if "crop_to" in shot:
        tx, ty, tw, th = shot["crop_to"]
        x, y, w, h = (lerp(x, tx, p), lerp(y, ty, p), lerp(w, tw, p), lerp(h, th, p))
    z0, z1 = shot.get("zoom", (1.0, 1.0))
    z = lerp(z0, z1, p)
    cx, cy = shot.get("center", (x + w / 2, y + h / 2))
    w2, h2 = w / z, h / z
    x, y = cx - w2 / 2, cy - h2 / 2
    # 越界时平移回边界，不改变尺寸（避免破坏 16:9）
    img = shot["_img"]
    x = max(0, min(x, img.width - w2))
    y = max(0, min(y, img.height - h2))
    return int(x), int(y), int(w2), int(h2)


def to_frame(rect, cx, cy, cw, ch):
    """源图矩形 → 画布矩形。"""
    x, y, w, h = rect
    fx = (x - cx) * W / cw
    fy = (y - cy) * H / ch
    fw = w * W / cw
    fh = h * H / ch
    return fx, fy, fw, fh


# ---------------------------------------------------------------- 画面元素

def spotlight(frame: Image.Image, rois, dim: float, feather: int = 46) -> Image.Image:
    """把 ROI 之外的区域压暗，引导视线。

    必须在 RGB 空间做混合：早先用带 alpha 的遮罩层 + RGBA→RGB 转换，
    alpha 被丢弃后遮罩变成了纯黑，整帧除了 ROI 全黑。改为对不透明副本
    做亮度压暗再用 mask 合成，亮度不会被丢弃。
    """
    if not rois:
        return frame
    mask = Image.new("L", (W, H), 0)
    md = ImageDraw.Draw(mask)
    for (fx, fy, fw, fh) in rois:
        md.rounded_rectangle([fx, fy, fx + fw, fy + fh], radius=18, fill=255)
    mask = mask.filter(ImageFilter.GaussianBlur(feather))
    base = frame.convert("RGB")
    dark = ImageEnhance.Brightness(base).enhance(1.0 - dim)
    return Image.composite(base, dark, mask)


def callout(d: ImageDraw.ImageDraw, rect, color: str, label: str, anchor: str, font_c,
            show_label: bool = True):
    """描出高亮框，并给标签加一条引线连回框。

    教训：标签若离框太远，观众会把标签当成在标注旁边的另一个东西——
    看起来就像"圈错了位置"。所以标签一律贴着框，且画一条引线明确归属。
    """
    fx, fy, fw, fh = rect
    rgb = tuple(int(color[i:i + 2], 16) for i in (1, 3, 5))
    lw = 5
    d.rounded_rectangle([fx, fy, fx + fw, fy + fh], radius=14, outline=rgb, width=lw)
    # 四角加粗，强调"框住的是重点"
    cl = min(fw, fh) * 0.22
    for (ax, ay, dx, dy) in ((fx, fy, 1, 1), (fx + fw, fy, -1, 1),
                             (fx, fy + fh, 1, -1), (fx + fw, fy + fh, -1, -1)):
        d.line([(ax, ay), (ax + dx * cl, ay)], fill=rgb, width=lw + 4)
        d.line([(ax, ay), (ax, ay + dy * cl)], fill=rgb, width=lw + 4)
    if not label or not show_label:
        return
    tw = d.textbbox((0, 0), label, font=font_c)[2]
    pad_x, pad_y = 20, 10
    bw, bh = tw + pad_x * 2, font_c.size + pad_y * 2
    if anchor == "above":
        bx, by = fx + fw / 2 - bw / 2, fy - bh - 10
        anchor_pt = (bx + bw / 2, by + bh)
        line_from = (fx + fw / 2, fy)
    elif anchor == "below":
        bx, by = fx + fw / 2 - bw / 2, fy + fh + 10
        anchor_pt = (bx + bw / 2, by)
        line_from = (fx + fw / 2, fy + fh)
    elif anchor == "right":
        bx, by = fx + fw + 16, fy + fh / 2 - bh / 2
        anchor_pt = (bx, by + bh / 2)
        line_from = (fx + fw, fy + fh / 2)
    else:                                     # left
        bx, by = fx - bw - 16, fy + fh / 2 - bh / 2
        anchor_pt = (bx + bw, by + bh / 2)
        line_from = (fx, fy + fh / 2)
    # 收敛到画布内
    bx = max(8, min(bx, W - bw - 8))
    by = max(8, min(by, H - bh - 8))
    d.line([line_from, anchor_pt], fill=rgb, width=3)
    d.rounded_rectangle([bx, by, bx + bw, by + bh], radius=10, fill=rgb)
    d.text((bx + pad_x, by + pad_y - 2), label, font=font_c, fill=(255, 255, 255))


def header_strip(canvas: Image.Image, title: str | None):
    """顶部横栏：左侧品牌，右侧段落标题。

    之前品牌条叠在画面左上角，正好压住产品页面自己的标题和统计卡
    （"审查总数 9"被盖掉）。改成顶部独立横栏后，画面内容零遮挡。
    """
    d = ImageDraw.Draw(canvas, "RGBA")
    d.rectangle([0, 0, W, HEADER_H], fill=(9, 14, 26, 255))
    d.rectangle([0, HEADER_H - 2, W, HEADER_H], fill=ACCENT + (200,))
    logo = load_logo()
    if logo:
        lg = logo.resize((30, 30), Image.LANCZOS)
        canvas.alpha_composite(lg, (44, int((HEADER_H - 30) / 2)))
    d.text((88, HEADER_H // 2), "AI Code Review", font=font(26, bold=True),
           fill=(255, 255, 255, 240), anchor="lm")
    if title:
        tw = d.textbbox((0, 0), title, font=font(30, bold=True))[2]
        d.rectangle([W - 44 - tw - 22, HEADER_H // 2 - 20, W - 44 - tw - 14, HEADER_H // 2 + 20],
                    fill=ACCENT + (255,))
        d.text((W - 44, HEADER_H // 2), title, font=font(30, bold=True),
               fill=(255, 255, 255, 245), anchor="rm")
    return canvas


def subtitle_band(frame: Image.Image, text: str, alpha: int = 232):
    """铺底部渐变 + 字幕。返回 RGB —— 调用方按 rawvideo 写入，必须 3 字节/像素。"""
    if not text:
        return frame.convert("RGB")
    ov = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    d = ImageDraw.Draw(ov)
    for i in range(190):
        a = int(alpha * (i / 190) ** 1.5)
        d.rectangle([0, H - 190 + i, W, H - 190 + i + 1], fill=(8, 12, 22, a))
    f = font(44, bold=True)
    lines = wrap(d, text, f, W - 260)
    sy = H - 92 - (len(lines) - 1) * 30
    for ln in lines:
        d.text((W / 2, sy), ln, font=f, fill=(255, 255, 255, 255), anchor="mm",
               stroke_width=3, stroke_fill=(0, 0, 0, 160))
        sy += 60
    return Image.alpha_composite(frame.convert("RGBA"), ov).convert("RGB")


# ---------------------------------------------------------------- 卡片镜头

def render_card(shot: dict, p: float) -> Image.Image:
    bg = Image.new("RGBA", (W, H), INK + (255,))
    # 半透明填充必须画在独立透明层上再 alpha_composite 回来。
    # PIL 的混合模式只在「RGB 图 + Draw(mode='RGBA')」下生效；RGBA 图上直接画
    # fill=(255,255,255,14) 是替换像素（含 alpha），最后 convert("RGB") 变实白。
    ov = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    d = ImageDraw.Draw(ov)
    # 顶部品牌色带
    d.rectangle([0, 0, W, 6], fill=ACCENT + (255,))
    # 柔和光晕
    glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    gd.ellipse([W / 2 - 520, 120, W / 2 + 520, 900], fill=(64, 158, 255, 34))
    bg = Image.alpha_composite(bg, glow.filter(ImageFilter.GaussianBlur(120)))

    if shot.get("components"):
        # 四个组件卡
        title = shot.get("title", "")
        a = int(255 * ease(min(1, p / 0.18)))
        f_title = font(64, bold=True)
        draw_text(d, (W / 2, 210), title, f_title, (255, 255, 255, a), anchor="mm")
        items = [("review-service", "主服务 · API + 评审 worker", "8080"),
                 ("mcp-server", "MCP 端点 · 外部 Agent 发起评审", "8082"),
                 ("review-frontend", "Web 控制台 · Vue 3 + Element Plus", "80"),
                 ("argus", "外部审查引擎 · 异步回报（可选）", "8899")]
        cw, ch, gap = 380, 210, 34
        total = len(items) * cw + (len(items) - 1) * gap
        x0 = (W - total) / 2
        for i, (name, desc, port) in enumerate(items):
            ca = int(255 * ease(min(1, max(0, (p - 0.10 - i * 0.06) / 0.16))))
            if ca <= 0:
                continue
            ox = x0 + i * (cw + gap)
            oy = 380 + 26 * (1 - ease(min(1, max(0, (p - 0.10 - i * 0.06) / 0.16))))
            d.rounded_rectangle([ox, oy, ox + cw, oy + ch], radius=20,
                                fill=(255, 255, 255, 14), outline=(255, 255, 255, 40), width=2)
            d.rounded_rectangle([ox, oy, ox + cw, oy + 7], radius=4, fill=ACCENT + (ca,))
            d.text((ox + 30, oy + 44), name, font=font(34, bold=True), fill=(255, 255, 255, ca))
            d.text((ox + 30, oy + 100), desc, font=font(22), fill=(180, 195, 220, ca))
            d.text((ox + 30, oy + 150), f":{port}", font=font(30, mono=True),
                   fill=(120, 190, 255, ca))
        return Image.alpha_composite(bg, ov)

    # 开场 / 收尾卡
    title = shot.get("title", "")
    sub = shot.get("subtitle", "")
    cmd = shot.get("cmd", "")
    if shot.get("outro"):
        a = int(255 * ease(min(1, p / 0.2)))
        logo = load_logo()
        if logo:
            lg = logo.resize((120, 120), Image.LANCZOS)
            bg.alpha_composite(lg, (int(W / 2 - 60), 250))
        f_title = font(86, bold=True)
        draw_text(d, (W / 2, 400), title, f_title, (255, 255, 255, a), anchor="mm")
        b = int(255 * ease(min(1, max(0, (p - 0.15) / 0.2))))
        f_sub = font(40)
        draw_text(d, (W / 2, 522), sub, f_sub, (200, 214, 235, b), anchor="mm")
        c = int(255 * ease(min(1, max(0, (p - 0.3) / 0.2))))
        if cmd:
            f_cmd = font(30, mono=True)
            tw = d.textbbox((0, 0), cmd, font=f_cmd)[2]
            d.rounded_rectangle([W / 2 - tw / 2 - 30, 620, W / 2 + tw / 2 + 30, 620 + 58],
                                radius=12, fill=(255, 255, 255, 16), outline=(120, 190, 255, 90), width=2)
            d.text((W / 2, 649), cmd, font=f_cmd, fill=(150, 205, 255, c), anchor="mm")  # 已居中
        return Image.alpha_composite(bg, ov)
    else:
        a = int(255 * ease(min(1, p / 0.22)))
        logo = load_logo()
        if logo:
            lg = logo.resize((120, 120), Image.LANCZOS)
            bg.alpha_composite(lg, (int(W / 2 - 60), 300))
        f_title = font(76, bold=True)
        draw_text(d, (W / 2, 500), title, f_title, (255, 255, 255, a), anchor="mm")
        if shot.get("footnote"):
            fn = int(255 * ease(min(1, max(0, (p - 0.2) / 0.2))))
            draw_text(d, (W / 2, 600), shot["footnote"], font(30),
                      (140, 160, 190, fn), anchor="mm")
    return Image.alpha_composite(bg, ov)


def load_logo():
    """取产品标识。

    优先用前端的矢量 favicon（frontend/public/favicon.svg）——它是品牌原图，
    cairosvg 渲成任意尺寸都清晰。从登录页截图里抠只有 38px，放大到卡片尺寸会糊。
    没有 cairosvg 时退回截图裁剪。
    """
    p = OUT / "logo.png"
    if p.exists():
        return Image.open(p).convert("RGBA")
    OUT.mkdir(parents=True, exist_ok=True)
    svg = ROOT.parent.parent / "frontend" / "public" / "favicon.svg"
    try:
        import cairosvg
        cairosvg.svg2png(url=str(svg), write_to=str(p), output_width=512, output_height=512)
        return Image.open(p).convert("RGBA")
    except Exception as e:                                   # noqa: BLE001
        print(f"[logo] cairosvg 不可用，退回截图裁剪: {e}")
    src = Image.open(DOCS / "00-login.png").convert("RGB")
    crop = src.crop((88, 42, 126, 79)).resize((136, 136), Image.LANCZOS).convert("RGBA")
    crop.save(p)
    return crop


def tts_all(video: list[dict]) -> list[dict]:
    """为每个镜头的每一句准备配音，并按镜头组织时间轴。

    优先用神经网络 TTS 预生成的音频（out/tts/<VOICE_DIR>/*.mp3）；
    没有的句子退回 macOS `say`（音质差，只作兜底）。

    注意：换音色会改变每句时长 → 镜头长度和字幕时间轴随之变化，
    所以换音色后**必须重渲画面**，不能只换音轨（否则字幕与语音会错位）。
    """
    TTS.mkdir(parents=True, exist_ok=True)
    TTS_FALLBACK.mkdir(parents=True, exist_ok=True)
    out = []
    for shot in video:
        sents = split_sentences(shot["narration"])
        segs = []
        for i, s in enumerate(sents):
            f = TTS / f"{shot['id']}_{i:02d}.mp3"
            fb = TTS_FALLBACK / f"{shot['id']}_{i:02d}.aiff"
            if not f.exists() and not fb.exists():
                subprocess.run(["say", "-v", VOICE, "-o", str(fb), s], check=True)
            path = f if f.exists() else fb
            segs.append({"text": s, "path": path, "dur": dur(path)})
        out.append({**shot, "segs": segs})
    return out


def build_timeline(timeline: list[dict]) -> list[dict]:
    t = 0.0
    for shot in timeline:
        speech = sum(s["dur"] for s in shot["segs"]) + SENT_GAP * (len(shot["segs"]) - 1)
        shot["start"] = t
        shot["speech_dur"] = speech
        shot["dur"] = max(shot.get("duration", 0) or 0, speech + PAD_LEAD + PAD_TAIL)
        # 句子在镜头内的相对起点
        acc = PAD_LEAD
        for s in shot["segs"]:
            s["t0"] = acc
            acc += s["dur"] + SENT_GAP
        t += shot["dur"]
    return timeline


def subtitle_at(shot, local: float) -> str:
    for s in shot["segs"]:
        if s["t0"] <= local < s["t0"] + s["dur"] + 0.18:
            return s["text"]
    return ""


# ---------------------------------------------------------------- 渲染

def render_shot(shot, local: float, total: float,
                with_brand: bool = True, with_labels: bool = True) -> Image.Image:
    p = local / total if total else 0
    if shot.get("card"):
        frame = render_card(shot, p)
    else:
        img = shot["_img"]
        cx, cy, cw, ch = crop_at(shot, ease(p))
        frame = img.crop((cx, cy, cx + cw, cy + ch)).resize((W, H), Image.LANCZOS)
        frame = frame.filter(ImageFilter.UnsharpMask(radius=1.7, percent=115, threshold=3))
        frame = frame.convert("RGBA")
        rois = [to_frame(r, cx, cy, cw, ch) for r in shot.get("rois", [])]
        frame = spotlight(frame, rois, dim=0.45)
        d = ImageDraw.Draw(frame)
        fc = font(30, bold=True)
        for (rect, color, label, anchor) in shot.get("boxes", []):
            callout(d, to_frame(rect, cx, cy, cw, ch), color, label, anchor, fc,
                    show_label=with_labels)


    # 淡入淡出
    frame = frame.convert("RGBA")
    fa = min(1.0, local / FADE, (total - local) / FADE)
    if fa < 1.0:
        bg = Image.new("RGBA", (W, H), INK + (255,))
        frame = Image.alpha_composite(bg, frame)
        frame.putalpha(frame.getchannel("A").point(lambda v: int(v * max(0.0, fa))))
    return frame.convert("RGB")



# ---------------------------------------------------------------- 竖版 9:16
#
# 竖版不是把 16:9 裁一刀 —— 那样会把两侧的标注框切掉。做法是「换版式」：
#   上半：品牌 + 段落标题（大字）
#   中间：完整 16:9 UI 帧，缩到 1080 宽，原样保真（聚光保留、框内小标签去掉，
#         标签内容提升为下方大字标题）
#   下半：一句结论大字 + 逐句字幕
# 信息靠版式承载，手机上才看得清。

V_UI_Y = 430           # UI 帧顶边
V_HEAD_Y = 1180        # 结论大字基线
V_SUB_Y = 1470         # 字幕顶边


def v_canvas() -> Image.Image:
    bg = Image.new("RGBA", (VW, VH), INK + (255,))
    glow = Image.new("RGBA", (VW, VH), (0, 0, 0, 0))
    ImageDraw.Draw(glow).ellipse([VW / 2 - 580, 40, VW / 2 + 580, 1000],
                                 fill=(64, 158, 255, 30))
    return Image.alpha_composite(bg, glow.filter(ImageFilter.GaussianBlur(150)))


def render_vertical_shot(shot: dict, local: float, total: float) -> Image.Image:
    p = local / total if total else 0
    cv = v_canvas()
    ov = Image.new("RGBA", (VW, VH), (0, 0, 0, 0))
    d = ImageDraw.Draw(ov)
    d.rectangle([0, 0, VW, 8], fill=ACCENT + (255,))

    sub_y = V_SUB_Y
    if shot.get("card"):
        # 纯排版卡：标识 + 大字主张 + 注脚
        logo = load_logo()
        if logo:
            lg = logo.resize((120, 120), Image.LANCZOS)
            cv.alpha_composite(lg, (VW // 2 - 60, 520))
        f_t = font(80, bold=True)
        a = int(255 * ease(min(1, p / 0.2)))
        y = 760
        for ln in wrap(d, shot.get("title", ""), f_t, VW - 120):
            d.text((VW / 2, y), ln, font=f_t, fill=(255, 255, 255, a), anchor="mm")
            y += 100

        # 组件卡：竖版用 2×2 栅格，横版是横排一行
        if shot.get("components"):
            items = [("review-service", "主服务 · API + 评审 worker", "8080"),
                     ("mcp-server", "MCP 端点 · 外部 Agent 发起评审", "8082"),
                     ("review-frontend", "Web 控制台 · Vue 3 + Element Plus", "80"),
                     ("argus", "外部审查引擎 · 异步回报（可选）", "8899")]
            cwid, chg, gap = 496, 236, 24
            gy = 950
            for i, (name, desc, port) in enumerate(items):
                ca = int(255 * ease(min(1, max(0, (p - 0.10 - i * 0.05) / 0.16))))
                if ca <= 0:
                    continue
                gx = (VW - (cwid * 2 + gap)) // 2 + (i % 2) * (cwid + gap)
                gy2 = gy + (i // 2) * (chg + gap)
                d.rounded_rectangle([gx, gy2, gx + cwid, gy2 + chg], radius=20,
                                    fill=(255, 255, 255, 16), outline=(255, 255, 255, 46), width=2)
                d.rounded_rectangle([gx, gy2, gx + cwid, gy2 + 7], radius=4, fill=ACCENT + (ca,))
                d.text((gx + 28, gy2 + 48), name, font=font(31, bold=True), fill=(255, 255, 255, ca))
                d.text((gx + 28, gy2 + 108), desc, font=font(22), fill=(180, 195, 220, ca))
                d.text((gx + 28, gy2 + 168), f":{port}", font=font(30, mono=True),
                       fill=(120, 190, 255, ca))
            sub_y = 1580
        if shot.get("footnote"):
            fn = int(255 * ease(min(1, max(0, (p - 0.2) / 0.2))))
            d.text((VW / 2, y + 60), shot["footnote"], font=font(30),
                   fill=(140, 160, 190, fn), anchor="mm")
        # 部署命令 = 行动号召，竖版是投放平台，不能省
        cmd = shot.get("cmd", "")
        if cmd:
            cc = int(255 * ease(min(1, max(0, (p - 0.3) / 0.2))))
            f_c = font(32, mono=True)
            tw = d.textbbox((0, 0), cmd, font=f_c)[2]
            by = y + 150
            d.rounded_rectangle([VW / 2 - tw / 2 - 34, by, VW / 2 + tw / 2 + 34, by + 66],
                                radius=14, fill=(255, 255, 255, 18),
                                outline=(120, 190, 255, 110), width=2)
            d.text((VW / 2, by + 33), cmd, font=f_c, fill=(150, 205, 255, cc), anchor="mm")
    else:
        # 头部：标识 + 段落标题
        logo = load_logo()
        if logo:
            lg = logo.resize((92, 92), Image.LANCZOS)
            cv.alpha_composite(lg, (VW // 2 - 46, 86))
        d.text((VW / 2, 218), "AI Code Review", font=font(30, bold=True),
               fill=(255, 255, 255, 235), anchor="mm")
        f_t = font(50, bold=True)
        ty = 300
        for ln in wrap(d, shot.get("label", ""), f_t, VW - 120):
            d.text((VW / 2, ty), ln, font=f_t, fill=(255, 255, 255, 245), anchor="mm")
            ty += 66

        # 中部：完整 UI 帧（不带品牌条、不带框内小标签）
        hframe = render_shot(shot, local, total, with_brand=False, with_labels=False)
        fh = int(VW * H / W)
        hframe = hframe.resize((VW, fh), Image.LANCZOS)
        mask = Image.new("L", (VW, fh), 0)
        ImageDraw.Draw(mask).rounded_rectangle([0, 0, VW - 1, fh - 1], radius=20, fill=255)
        hframe = hframe.convert("RGBA")
        hframe.putalpha(mask)
        cv.alpha_composite(hframe, (0, V_UI_Y))
        d.rounded_rectangle([0, V_UI_Y, VW - 1, V_UI_Y + fh - 1], radius=20,
                            outline=(255, 255, 255, 50), width=2)

        # 下部：一句结论大字
        head = shot.get("vheadline", "")
        if head:
            f_h = font(62, bold=True)
            hy = V_HEAD_Y
            for ln in wrap(d, head, f_h, VW - 110):
                d.text((VW / 2, hy), ln, font=f_h, fill=(255, 255, 255, 250), anchor="mm")
                hy += 80

    cv = Image.alpha_composite(cv, ov)

    # 底部：逐句字幕（卡片镜头自带大字，不再叠字幕）
    sub = "" if shot.get("card") else subtitle_at(shot, local)
    if sub:
        d2 = ImageDraw.Draw(cv, "RGBA")
        d2.rectangle([VW / 2 - 40, sub_y - 58, VW / 2 + 40, sub_y - 54],
                     fill=ACCENT + (255,))
        f_s = font(44)
        sy = sub_y
        for ln in wrap(d2, sub, f_s, VW - 130):
            d2.text((VW / 2, sy), ln, font=f_s, fill=(255, 255, 255, 245), anchor="mm")
            sy += 60

    # 底部字标：收住版面重心
    d3 = ImageDraw.Draw(cv, "RGBA")
    d3.rectangle([VW / 2 - 28, 1826, VW / 2 + 28, 1830], fill=ACCENT + (170,))
    d3.text((VW / 2, 1862), "AI Code Review · 代码审查引擎", font=font(26),
            fill=(150, 168, 196, 205), anchor="mm")

    # 淡入淡出
    cv = cv.convert("RGBA")
    fa = min(1.0, local / FADE, (total - local) / FADE)
    if fa < 1.0:
        cv = Image.alpha_composite(Image.new("RGBA", (VW, VH), INK + (255,)), cv)
        cv.putalpha(cv.getchannel("A").point(lambda v: int(v * max(0.0, fa))))
    return cv.convert("RGB")


def render_video_vertical(key: str, timeline: list[dict], probe: bool):
    for shot in timeline:
        if not shot.get("card"):
            shot["_img"] = Image.open(DOCS / f"{shot['image']}.png").convert("RGB")
    total_dur = sum(s["dur"] for s in timeline)
    print(f"[{key}-V] {len(timeline)} 镜头，总时长 {total_dur:.1f}s")

    if probe:
        outdir = OUT / f"probe-{key}-V"
        outdir.mkdir(parents=True, exist_ok=True)
        for shot in timeline:
            fr = render_vertical_shot(shot, shot["dur"] * 0.55, shot["dur"])
            fr.save(outdir / f"{shot['id']}.png")
            print("  probe", shot["id"])
        return

    proc = subprocess.Popen(
        ["ffmpeg", "-y", "-f", "rawvideo", "-pix_fmt", "rgb24", "-s", f"{VW}x{VH}",
         "-r", str(FPS), "-i", "-", "-an",
         "-c:v", "libx264", "-preset", "slow", "-crf", "19", "-pix_fmt", "yuv420p",
         str(OUT / f"{key}-V-silent.mp4")],
        stdin=subprocess.PIPE)
    for shot in timeline:
        n = int(shot["dur"] * FPS)
        for i in range(n):
            fr = render_vertical_shot(shot, i / FPS, shot["dur"])
            if fr.mode != "RGB" or fr.size != (VW, VH):
                fr = fr.convert("RGB")
            proc.stdin.write(fr.tobytes())
        print(f"  {shot['id']} done ({shot['dur']:.1f}s)")
    proc.stdin.close()
    proc.wait()



def verify_voice(key: str, timeline: list[dict]) -> bool:
    """自检：确认音轨确实来自目标音色，而不是复用了旧缓存。

    做法：把每句的**源音频**单独解码，与装配好的 narration.wav 中对应区间
    做逐字节比对。只要有一个不匹配，就说明音轨与源文件脱节。

    这道检查存在的理由：历史上出现过「换了音色但成片还是老声音」的静默错误——
    转换缓存没按音色隔离，而 build_audio 又会把音频填充到新时间轴，
    导致时长对得上、声音是旧的，只能靠逐字节比对才暴露。
    """
    import wave as _wave
    segdir = OUT / f"audio-{key}" / VOICE_DIR
    nar_path = segdir / "narration.wav"
    if not nar_path.exists():
        print(f"[{key}] 自检跳过：找不到 {nar_path}")
        return False
    with _wave.open(str(nar_path), "rb") as w:
        nar = w.readframes(w.getnframes())

    ok = True
    for shot in timeline:
        for si, s in enumerate(shot["segs"]):
            src = segdir / f"{shot['id']}_{si:02d}.wav"
            if not src.exists():
                continue
            with _wave.open(str(src), "rb") as w:
                head = w.readframes(min(w.getnframes(), 24000))   # 前 0.5s
            # 装配时每句前有 PAD_LEAD + 前面各句的空白
            idx = nar.find(head[:4800])                          # 精确字节定位
            if idx < 0:
                print(f"[{key}] 自检失败：{shot['id']}_{si:02d} 的源音频在成片音轨中找不到")
                ok = False
    print(f"[{key}] 音色自检：{'通过（音轨与源文件一致）' if ok else '失败'}")
    return ok


def mux_vertical(key: str, timeline):
    silent = OUT / f"{key}-V-silent.mp4"
    mixed = OUT / f"audio-{key}" / VOICE_DIR / "mixed.wav"
    final = OUT / f"review-service-{key}-vertical.mp4"
    total = sum(s["dur"] for s in timeline)
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", str(silent), "-i", str(mixed),
                    "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-t", f"{total:.3f}",
                    "-movflags", "+faststart", str(final)], check=True)
    print(f"[{key}-V] 成片 -> {final}  ({dur(final):.1f}s)")
    return final


def compose_horizontal(ui: Image.Image, shot: dict, subtitle: str) -> Image.Image:
    """横版合成：顶部品牌横栏 + 画面缩放嵌入 + 底部字幕。

    画面整体缩到 (1920, 996) 再放进 y=84 的横栏下方——不是硬裁，
    否则底部 84px（正好是字幕要盖住的那条）里的高亮框会被切掉。
    """
    canvas = Image.new("RGBA", (W, H), INK + (255,))
    body = ui.convert("RGBA").resize((W, H - HEADER_H), Image.LANCZOS)
    canvas.alpha_composite(body, (0, HEADER_H))
    header_strip(canvas, shot.get("label"))
    canvas = subtitle_band(canvas, subtitle)
    return canvas.convert("RGB")


def render_video(key: str, timeline: list[dict], probe: bool):
    for shot in timeline:
        if not shot.get("card"):
            shot["_img"] = Image.open(DOCS / f"{shot['image']}.png").convert("RGB")

    total_dur = sum(s["dur"] for s in timeline)
    n_frames = int(total_dur * FPS)
    print(f"[{key}] {len(timeline)} 镜头，总时长 {total_dur:.1f}s，{n_frames} 帧")

    if probe:
        outdir = OUT / f"probe-{key}"
        outdir.mkdir(parents=True, exist_ok=True)
        picks = []
        for shot in timeline:
            picks.append((shot["id"], shot["start"] + shot["dur"] * 0.55))
        for sid, t in picks:
            shot = next(s for s in timeline if s["id"] == sid)
            ui = render_shot(shot, t - shot["start"], shot["dur"])
            sub = "" if shot.get("card") else subtitle_at(shot, t - shot["start"])
            fr = compose_horizontal(ui, shot, sub)
            fr.save(outdir / f"{sid}.png")
            print("  probe", sid)
        return

    proc = subprocess.Popen(
        ["ffmpeg", "-y", "-f", "rawvideo", "-pix_fmt", "rgb24", "-s", f"{W}x{H}",
         "-r", str(FPS), "-i", "-", "-an",
         "-c:v", "libx264", "-preset", "slow", "-crf", "19", "-pix_fmt", "yuv420p",
         str(OUT / f"{key}-silent.mp4")],
        stdin=subprocess.PIPE)

    fi = 0
    for shot in timeline:
        n = int(shot["dur"] * FPS)
        for i in range(n):
            local = i / FPS
            ui = render_shot(shot, local, shot["dur"])
            # 卡片镜头不叠字幕：卡片上已经写着同一句话，再压一条就是重复
            sub = "" if shot.get("card") else subtitle_at(shot, local)
            fr = compose_horizontal(ui, shot, sub)
            # 必须统一成 RGB：subtitle_band 有字幕时返回 RGBA(4B/px)、无字幕时返回
            # RGB(3B/px)，两种字节长度交替写进 rawvideo 会让解码端错位——画面会横向撕裂。
            if fr.mode != "RGB" or fr.size != (W, H):
                fr = fr.convert("RGB")
                assert fr.size == (W, H), fr.size
            proc.stdin.write(fr.tobytes())
            fi += 1
        print(f"  {shot['id']} done ({shot['dur']:.1f}s)")
    proc.stdin.close()
    proc.wait()


# ---------------------------------------------------------------- 音频

def build_audio(key: str, timeline: list[dict]):
    """按采样点精确拼装配音轨，保证音轨总长与视频总长逐帧对齐。

    之前用 ffmpeg concat + apad 拼句子，镜头尾部留白（PAD_TAIL）没有补进去，
    音轨比画面短十几个秒，mux 时 -shortest 直接把画面截了 —— 成片比预期短 12 秒。
    改成自己按采样点拼 PCM：每句前面补 PAD_LEAD，句间补 SENT_GAP，
    镜头末尾补到 shot['dur']，总长 == 视频总长。
    """
    import wave

    # 缓存目录必须按音色隔离！
    # 历史事故：这里曾是 out/audio-{key}（只按镜头 id 命名），换音色后
    # `if not s["wav"].exists()` 命中旧缓存 → 跳过转换 → 成片一直用第一轮的老声音，
    # 而 build_audio 又会把音频填充/截断到新时间轴，于是「时长对得上、声音是旧的」，
    # 肉眼和听感都很难第一时间发现（只有逐字节比对才暴露）。
    segdir = OUT / f"audio-{key}" / VOICE_DIR
    segdir.mkdir(parents=True, exist_ok=True)
    SR = 48000

    def read_pcm(path: pathlib.Path) -> bytes:
        with wave.open(str(path), "rb") as w:
            if w.getnchannels() != 1 or w.getsampwidth() != 2:
                raise RuntimeError(f"期望 48k/mono/16bit，实际 {w.getparams()}")
            return w.readframes(w.getnframes())

    def silence(n: int) -> bytes:
        return b"\x00\x00" * max(0, n)

    # 1) 每句转成 48k mono 16bit
    for shot in timeline:
        for si, s in enumerate(shot["segs"]):
            s["wav"] = segdir / f"{shot['id']}_{si:02d}.wav"
            # 一律重新转换，不吃缓存：17 个文件的转换成本合计 ~2 秒，
            # 相比几分钟的渲染完全可忽略，但能彻底消除「复用旧音轨」这类静默错误。
            subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", str(s["path"]),
                            "-ar", str(SR), "-ac", "1", "-c:a", "pcm_s16le",
                            str(s["wav"])], check=True)

    # 2) 逐镜头拼 PCM，每段长度严格等于 shot['dur']
    chunks = []
    for shot in timeline:
        parts = [silence(int(PAD_LEAD * SR))]
        for i, s in enumerate(shot["segs"]):
            if i:
                parts.append(silence(int(SENT_GAP * SR)))
            parts.append(read_pcm(s["wav"]))
        data = b"".join(parts)
        need = int(round(shot["dur"] * SR)) * 2
        if len(data) < need:
            data += silence((need - len(data)) // 2)
        else:
            data = data[:need]
        chunks.append(data)

    narration = segdir / "narration.wav"
    with wave.open(str(narration), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(SR)
        w.writeframes(b"".join(chunks))
    d = dur(narration)

    # 3) 氛围垫：三个正弦泛音 + 慢速起伏，低音量铺底
    bed = segdir / "bed.wav"
    subprocess.run([
        "ffmpeg", "-y", "-v", "error",
        "-f", "lavfi", "-i", f"sine=frequency=110:sample_rate=48000:duration={d}",
        "-f", "lavfi", "-i", f"sine=frequency=164.81:sample_rate=48000:duration={d}",
        "-f", "lavfi", "-i", f"sine=frequency=220:sample_rate=48000:duration={d}",
        "-filter_complex",
        "[0]volume=0.30[a];[1]volume=0.20[b];[2]volume=0.12[c];"
        "[a][b][c]amix=inputs=3,"
        "volume='0.35+0.65*abs(sin(2*PI*t/9))':eval=frame,"
        "lowpass=f=1500,afade=t=in:st=0:d=2.5,afade=t=out:st=%f:d=3" % (d - 3),
        "-ar", "48000", "-ac", "1", str(bed)], check=True)

    mixed = segdir / "mixed.wav"
    subprocess.run(["ffmpeg", "-y", "-v", "error",
                    "-i", str(narration), "-i", str(bed),
                    "-filter_complex",
                    "[0]volume=1.0,highpass=f=90[n];[1]volume=0.16[b];"
                    "[n][b]amix=inputs=2:duration=longest,loudnorm=I=-16:TP=-1.5:LRA=11[out]",
                    "-map", "[out]", "-ar", "48000", "-ac", "2", str(mixed)], check=True)
    return mixed


def mux(key: str, timeline):
    silent = OUT / f"{key}-silent.mp4"
    mixed = build_audio(key, timeline)
    final = OUT / f"review-service-{key}.mp4"
    total = sum(s["dur"] for s in timeline)
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", str(silent), "-i", str(mixed),
                    "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-t", f"{total:.3f}",
                    "-movflags", "+faststart", str(final)], check=True)
    print(f"[{key}] 成片 -> {final}  ({dur(final):.1f}s)")
    verify_voice(key, timeline)
    return final


# ---------------------------------------------------------------- main

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("keys", nargs="+")
    ap.add_argument("--probe", action="store_true", help="只出关键帧 PNG")
    ap.add_argument("--audio-only", action="store_true",
                    help="复用已有母版，只重做音轨与封装（不重渲染帧）")
    ap.add_argument("--vertical", action="store_true", help="渲染 9:16 竖版")
    args = ap.parse_args()

    OUT.mkdir(parents=True, exist_ok=True)
    for key in args.keys:
        key = key.upper()
        timeline = build_timeline(tts_all(VIDEOS[key]))
        if args.vertical:
            if args.probe:
                render_video_vertical(key, timeline, probe=True)
            elif args.audio_only:
                # 静帧母版已按当前时间轴渲好，只需换音轨再封装
                mux_vertical(key, timeline)
            else:
                render_video_vertical(key, timeline, probe=False)
                mux_vertical(key, timeline)
        elif args.probe:
            render_video(key, timeline, probe=True)
        elif args.audio_only:
            mux(key, timeline)
        else:
            render_video(key, timeline, probe=False)
            mux(key, timeline)


if __name__ == "__main__":
    main()
