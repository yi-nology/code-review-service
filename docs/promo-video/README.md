# 宣传视频制作工程

从**本地真实运行的控制台**生成两支宣传片（横版 + 竖版共四支）。
脚本见 [`script.md`](script.md)，产品功能说明见 [`../FUNCTIONAL_OVERVIEW.md`](../FUNCTIONAL_OVERVIEW.md)。

## 产物

| 文件 | 说明 |
|---|---|
| `out/review-service-A.mp4` | 主片《把代码评审变成可追溯的合并门禁》/ 1920×1080 |
| `out/review-service-B.mp4` | 副片《它凭什么敢接进生产》/ 1920×1080 |
| `out/review-service-A-vertical.mp4` | 主片竖版 / 1080×1920（视频号 / 抖音 / 小红书） |
| `out/review-service-B-vertical.mp4` | 副片竖版 / 1080×1920 |
| `out/*-silent.mp4` | 母版（无音轨，CRF 19） |

## 完整跑一遍

```sh
# 1) 起数据库与后端（后端连 ai_code_review_demo 库）
docker exec difftest-pg psql -U postgres -c "CREATE DATABASE ai_code_review_demo"
DATABASE_URL="postgres://postgres:postgres@localhost:15440/ai_code_review_demo?sslmode=disable" \
  REVIEW_TOKEN=dev-token go run . &
# 2) 灌演示数据
docker exec -i difftest-pg psql -U postgres -d ai_code_review_demo < scripts/seed-demo.sql
# 3) 前端 dev server 起在 :3000（frontend/ 目录下 npm run dev）
# 4) 抓产品截图 → docs/assets/screenshots/
node docs/promo-video/tools/capture.mjs
# 5) 生成配音（首次或改口播后）
python3 docs/promo-video/tools/gen_voice.py v_xiaoxiao zh-CN-XiaoxiaoNeural
# 6) 渲染
cd docs/promo-video
python3 tools/build.py A B                # 横版
python3 tools/build.py A B --vertical     # 竖版
```

## 常用开关

```sh
python3 tools/build.py A B --probe        # 只出关键帧 PNG，快速看构图（不编码视频）
python3 tools/build.py A B --audio-only   # 只重做音轨与封装，复用已有母版
PROMO_VOICE=v_yunxi python3 tools/build.py A B   # 换音色
bash tools/deliver.sh A B                 # 压投放版（CRF 26 + 7Mbps 上限，带 faststart）
```

## 依赖

- Python 3 + Pillow（逐帧合成、字幕烧录）
- ffmpeg（含 libx264 / aac）
- Node + `playwright-core`（装在 `frontend/node_modules` 下；脚本按绝对路径动态导入）
- **edge-tts**（配音）：`python3 -m venv /tmp/ttsenv && /tmp/ttsenv/bin/pip install edge-tts`
  （PEP 668 禁止装进系统 python，必须用 venv）
- **cairosvg**（矢量 logo）：`pip3 install --user cairosvg` 或用 venv
- 中文字体：Hiragino Sans GB（`/System/Library/Fonts/`）

## 结构

```
tools/shots.py       镜头编排（裁切矩形、聚光区域、标注框、口播、竖版结论）——改这个就行
tools/build.py       渲染器：配音 → 时间轴 → 逐帧合成 → ffmpeg 编码 → 混音封装
tools/capture.mjs    抓产品截图（Playwright）
tools/gen_voice.py   用 edge-tts 生成某音色的全部配音
tools/grid.py        给截图生成带坐标网格的缩略图，用来精确定位标注框
tools/deliver.sh     压投放版
```

## 横版版式

```
┌────────────────────────────────────────────┐
│ ● AI Code Review              段落标题   ▌ │  ← 84px 独立横栏
├────────────────────────────────────────────┤
│                                            │
│           16:9 产品画面（整体缩放嵌入）     │
│                                            │
│         ┌──────────┐                       │
│         │ 高亮框 + 引线标签 │               │
│         └──────────┘                       │
│  ══════════════════════════════════════   │
│              逐句字幕（硬烧）              │
└────────────────────────────────────────────┘
```

要点：
- **品牌横栏独立成条**，不叠在画面左上角——叠着会盖住页面自己的标题和统计卡
  （"审查总数 9"被盖掉是实际踩过的坑）。
- **标注标签必须贴框并画引线**。标签飘在框旁边几十像素处，观众会以为它在标注
  旁边的另一个东西，看起来就像"圈错了位置"。
- **卡片镜头不叠字幕**：卡片上已经写着同一句话，再压一条就是重复。

## 竖版（9:16）怎么做的

竖版**不是把 16:9 裁一刀**——那样会把两侧的标注框切掉。做法是换版式：

```
┌──────────────────┐
│   标识 + 段落标题  │
├──────────────────┤
│                  │
│   完整 16:9 UI 帧 │  ← 缩到 1080 宽，聚光保留、框内小标签去掉
│                  │
├──────────────────┤
│  一句结论（大字） │  ← vheadline 字段，原来框里的小标签提升到这里
│      逐句字幕     │
│  底部字标         │
└──────────────────┘
```

## 配音

配音源是 **Azure 神经音色**（edge-tts 直连），不是 macOS `say`——`say` 的中文只有婷婷
一个正常音色，其余是 Eddy/Flo 这类搞怪音色的，做宣传片不够用。

| 音色目录 | edge-tts 音色 id | 风格 |
|---|---|---|
| `v_xiaoxiao` | `zh-CN-XiaoxiaoNeural` | 温暖女声（**当前默认**） |
| `v_yunxi` | `zh-CN-YunxiNeural` | 阳光青年男声 |
| `v_yunyang` | `zh-CN-YunyangNeural` | 专业播报男声 |

> ⚠️ **换音色必须重渲画面，不能只换音轨。**
> 换音色后每句时长会变（实测最大偏差 58%），镜头长度和字幕时间轴都由音频时长推导，
> 只换音轨会让字幕与语音错位。

> 曾试过把新配音用 `atempo` 压回旧时长以复用母版（省一次重渲），
> 但实测偏差太大——「失败不静默。」旧版只用了 1.34s（压着说完的），神经音色 2.11s 才是正常语速，
> 硬拉会变花栗鼠。**为了声音自然，重渲是值得的。**

## ⚠️ 两个会静默出错的坑（都已修，勿改回）

**1. 音频转换缓存必须按音色隔离。**
`build_audio` 会把每句配音转成 48k/mono/16bit 的 wav 供拼接。曾写成
`out/audio-{key}/`（**只按镜头 id 命名**），换音色后 `if not exists` 命中旧缓存 →
跳过转换 → 成片一直用**第一轮的老声音**。
而 `build_audio` 又会把音频填充/截断到新时间轴，于是**时长对得上、声音是旧的**，
听感和肉眼看片都很难立刻发现（本次是靠逐字节比对才暴露的）。
现在：缓存落在 `out/audio-{key}/<音色>/` 下，且**一律重新转换**（19 个文件 ~2 秒，
相比几分钟的渲染可忽略），彻底消除这类静默错误。

**2. 渲染完必须验音色，不能只看时长。**
`build.py` 的 `mux()` 会自动跑 `verify_voice()`：把每句源音频在装配好的
`narration.wav` 里做**逐字节定位**，找不到就报错。手动复核用：
`ffprobe` 看时长**不能**证明音色对——用逐字节或包络相关性比对。

同理，`gen_voice.py` 也**一律重新生成**音频：口播文案改了但文件名没变时，
跳过会留下旧音频 —— 与上面是同一类静默错误。
