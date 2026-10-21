"""镜头定义 —— AI Code Review 宣传片的全部编排数据。

坐标系一律是**源图像素**（新截图 1920 宽）。build.py 负责映射到 1920x1080 画布。

crop   : (x, y, w, h) 源图裁切矩形，h 固定为 w*9/16（16:9）
crop_to: 可选，镜头结束时裁切矩形（用于平移/推拉）
zoom   : (起, 止) 额外缩放系数，在 crop 中心放大
center : 缩放中心（源图坐标）
rois   : 需要聚光高亮的区域（源图坐标）
boxes  : 描边标注框 [(rect, color, label, label_anchor), ...]
label  : 段落小标题（画在左上角品牌条下方）
card   : True 表示这是一张纯排版卡片，不使用截图
"""

# ---------------------------------------------------------------- 主片 A
A_VIDEO = [
    dict(
        id="A1", card=True, duration=6.0,
        title="代码评审，还在靠人盯人？",
        footnote="AI Code Review · 代码审查引擎",
        narration="代码评审，还在靠人盯人？",
    ),
    dict(
        id="A2", image="02-reviews-list",
        crop=(280, 230, 1220, 686), crop_to=(280, 520, 1220, 686),
        zoom=(1.0, 1.05), center=(900, 500),
        label="审查记录",
        vheadline="结论直接写在列表上",
        rois=[(1235, 560, 90, 330)],
        boxes=[((1235, 560, 90, 330), "#67c23a", "门禁结论直出", "right")],
        narration="现在，每一次评审的结论，都直接写在列表上——不用点进去才知道结果。",
    ),
    dict(
        id="A3", image="03-review-detail",
        crop=(285, 285, 720, 405),
        zoom=(1.0, 1.03), center=(520, 480),
        label="审查详情",
        vheadline="为什么没通过、扣在哪分，一屏说清",
        rois=[(295, 295, 355, 230)],
        boxes=[((295, 295, 355, 230), "#f56c6c", "门禁未通过 · 80/100", "right")],
        narration="为什么没通过，扣在哪分，先修哪一条，一屏说清楚。",
    ),
    dict(
        id="A4", image="03-review-detail",
        # 取景放宽到 900 源像素宽：时间线右半是空白，正好用来放引出的标注，
        # 同时把带耗时的条目抬到画面中部（不被底部字幕带盖住）。
        crop=(596, 250, 900, 506),
        zoom=(1.0, 1.0), center=(1046, 503),
        label="审查详情 · 执行轨迹",
        vheadline="每一步、每一毫秒，都有迹可循",
        rois=[(648, 282, 90, 44), (700, 485, 300, 55)],
        boxes=[((700, 485, 300, 55), "#909399", "仓库准备 · 771ms", "right")],
        narration="从入队到写回，十四个步骤，每一步、每一毫秒都有迹可循。",
    ),
    dict(
        id="A5", image="06-settings-platforms",
        # 取景聚焦平台列表：源图里「已配置 8」(y245) 与「Gerrit 行」(y640) 相距 440px，
        # 16:9 裁切装不下两者——硬凑会把 Gerrit 压进底部字幕带里。
        # 改用列表面板的「显示 8/8」作为"八个平台"的证据，Gerrit 落在画面中部。
        crop=(280, 430, 1173, 660),
        zoom=(1.0, 1.03), center=(860, 700),
        label="多平台接入",
        vheadline="八个平台，一个入口",
        # 「显示 8 / 8」只做聚光不贴标签：它就在画面左上角，
        # 标签会压住数字本身，反而看不清。
        rois=[(288, 428, 90, 26), (305, 640, 250, 45)],
        boxes=[
            ((305, 640, 250, 45), "#67c23a", "Gerrit：独立客户端，全链路打通", "right"),
        ],
        narration="GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode、腾讯工蜂、Gerrit——八个平台，一个入口。",
    ),
    dict(
        id="A6", image="05-settings-engine",
        crop=(280, 640, 800, 450),
        zoom=(1.0, 1.03), center=(540, 860),
        label="审查引擎",
        vheadline="密钥只写不读，读取一律掩码",
        rois=[(295, 740, 200, 60)],
        boxes=[((295, 740, 200, 60), "#67c23a", "API Key：读取一律返回掩码", "right")],
        narration="模型、密钥、并发、工具、版本，全在控制台里配——密钥只写不读，读取一律掩码。",
    ),
    dict(
        id="A7", image="07-settings-agents",
        crop=(280, 330, 900, 506), crop_to=(280, 380, 900, 506),
        zoom=(1.0, 1.03), center=(560, 520),
        label="审查代理",
        vheadline="按技术栈自动匹配审查员",
        rois=[(290, 520, 200, 95)],
        boxes=[((290, 520, 200, 95), "#67c23a", "按技术栈自动匹配", "right")],
        narration="按技术栈自动匹配审查员，安全和性能，永远在。",
    ),
    dict(
        id="A8", image="10-project-reports",
        crop=(400, 200, 1200, 675),
        zoom=(1.0, 1.04), center=(1000, 320),
        label="项目报告",
        vheadline="质量与花销，同一张表里看",
        rois=[(1438, 250, 80, 52)],
        boxes=[((1438, 250, 80, 52), "#e6a23c", "Token 总量 · 成本与质量同表", "above")],
        narration="质量和花销，放在同一张表里看。",
    ),
    dict(
        id="A9", image="11-diagnostics",
        crop=(280, 200, 1500, 844),
        zoom=(1.0, 1.04), center=(900, 420),
        label="诊断中心",
        vheadline="失败不静默，异常可查",
        rois=[(288, 350, 700, 95)],
        boxes=[((288, 350, 700, 95), "#f56c6c", "写回失败：写明原因", "below")],
        narration="评论没发出去，诊断中心能查到底。失败不静默。",
    ),
    dict(
        id="A10", card=True, duration=9.0, outro=True,
        title="AI Code Review",
        subtitle="把代码评审，从人盯人，变成机器守门。",
        cmd="docker compose up --build",
        narration="AI Code Review，把代码评审，从人盯人，变成机器守门。",
    ),
]


# ---------------------------------------------------------------- 副片 B
B_VIDEO = [
    dict(
        id="B1", card=True, duration=6.0, components=True,
        title="一套 compose，四个组件",
        narration="一套 compose，四个组件，控制台和后端分开部署。",
    ),
    dict(
        id="B2", image="06-settings-platforms",
        crop=(290, 560, 560, 315), crop_to=(290, 600, 560, 315),
        zoom=(1.0, 1.06), center=(430, 690),
        label="多平台 · 含 Gerrit",
        vheadline="两套引擎，三种审查模式",
        rois=[(305, 640, 250, 45)],
        boxes=[((305, 640, 250, 45), "#67c23a", "Gerrit：独立客户端，全链路打通", "right")],
        narration="Gerrit 也在——它不依赖通用 SDK，走的是独立客户端，clone、webhook、回写全链路都在。",
    ),
    dict(
        id="B3", image="03-review-detail",
        crop=(0, 60, 620, 349), crop_to=(0, 120, 620, 349),
        zoom=(1.0, 1.04), center=(260, 260),
        label="两套引擎 · 三种模式",
        vheadline="OCR / Argus / 全库扫描",
        rois=[(0, 150, 250, 170)],
        boxes=[((0, 150, 250, 170), "#409eff", "OCR / Argus / 全库扫描", "right")],
        narration="两套审查引擎随时切：OCR 多智能体、Argus 外部引擎，还能做全库扫描。",
    ),
    dict(
        id="B4", image="09-settings-projects",
        crop=(600, 500, 1200, 675),
        zoom=(1.0, 1.02), center=(1200, 830),
        label="质量门禁 · 四档",
        vheadline="门禁四档，逐仓库定",
        rois=[(600, 545, 1180, 215)],
        boxes=[((610, 548, 110, 42), "#f56c6c", "门禁四档 · 逐仓库定", "above")],
        narration="门禁四档逐仓库定：只拦严重、拦高风险及以上、拦中风险及以上，或者全拦。",
    ),
    dict(
        id="B5", image="02-reviews-list",
        crop=(280, 540, 1220, 686),
        zoom=(1.0, 1.06), center=(900, 700),
        label="状态落到 commit",
        vheadline="红绿由平台判定",
        rois=[(1240, 570, 80, 320)],
        boxes=[((1240, 570, 80, 320), "#67c23a", "红绿由平台判定", "right")],
        narration="门禁结论落到 commit 上，红还是绿，由平台判定。",
    ),
    dict(
        id="B6", image="11-diagnostics",
        crop=(280, 200, 1400, 788), crop_to=(280, 420, 1400, 788),
        zoom=(1.0, 1.04), center=(900, 520),
        label="失败不静默",
        vheadline="写回失败，写明原因",
        rois=[(288, 350, 700, 95)],
        boxes=[((288, 350, 700, 95), "#f56c6c", "写回失败：写明原因", "below")],
        narration="写回降级、Agent 失败、门禁未过，都写明原因。",
    ),
    dict(
        id="B7", image="10-project-reports",
        crop=(280, 1000, 1400, 788), crop_to=(280, 1180, 1400, 788),
        zoom=(1.0, 1.05), center=(900, 1400),
        label="项目质量报告",
        vheadline="每个项目的风险分布都摊开",
        rois=[(1245, 1240, 320, 200)],
        boxes=[((1245, 1240, 320, 200), "#409eff", "平均分 · 通过率 · 风险分布", "above")],
        narration="每个项目的风险分布，都摊开给你看。",
    ),
    dict(
        id="B8", card=True, duration=8.0, outro=True,
        title="AI Code Review",
        subtitle="接入只要一条命令，剩下的，交给机器。",
        cmd="docker compose up --build",
        narration="接入只要一条命令，剩下的，交给机器。",
    ),
]


VIDEOS = {"A": A_VIDEO, "B": B_VIDEO}
