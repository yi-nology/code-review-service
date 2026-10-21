#!/usr/bin/env python3
"""给截图生成带坐标网格的缩略图，用于精确标注。输出到 out/grid/<name>.png"""
import sys, pathlib
from PIL import Image, ImageDraw, ImageFont

SRC = pathlib.Path(__file__).resolve().parent.parent.parent.parent / "docs" / "assets" / "screenshots"
OUT = pathlib.Path(__file__).resolve().parent.parent / "out" / "grid"
FONT = "/System/Library/Fonts/Hiragino Sans GB.ttc"

def build(name, maxw=760):
    im = Image.open(SRC / f"{name}.png").convert("RGB")
    sw, sh = im.size
    scale = maxw / sw
    im = im.resize((int(sw*scale), int(sh*scale)), Image.LANCZOS)
    d = ImageDraw.Draw(im)
    f = ImageFont.truetype(FONT, 11, index=0)
    step = 100  # 源图坐标步长
    for x in range(0, sw+1, step):
        X = int(x*scale)
        d.line([(X,0),(X,im.height)], fill=(255,0,0), width=1)
        d.text((X+2, 2), str(x), font=f, fill=(255,0,0))
    for y in range(0, sh+1, step):
        Y = int(y*scale)
        d.line([(0,Y),(im.width,Y)], fill=(255,0,0), width=1)
        d.text((2, Y+2), str(y), font=f, fill=(255,0,0))
    OUT.mkdir(parents=True, exist_ok=True)
    p = OUT / f"{name}.png"
    im.save(p)
    print(p, im.size, "source", (sw, sh))

if __name__ == "__main__":
    for n in sys.argv[1:]:
        build(n)
