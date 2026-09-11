# -*- coding: utf-8 -*-
"""Generate documentation PNGs with callout badges (GitHub-friendly)."""
from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).resolve().parent / "images"
OUT.mkdir(parents=True, exist_ok=True)

W, H = 960, 540
BG = (244, 247, 251)
CARD = (255, 255, 255)
NAVY = (23, 50, 79)
MUTED = (91, 107, 124)
ACCENT = (232, 93, 76)
BLUE = (43, 108, 176)
GREEN = (47, 158, 107)
ORANGE = (221, 107, 32)
RED = (229, 62, 62)
LINE = (227, 235, 243)
SIDE = (23, 50, 79)


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    candidates = [
        r"C:\Windows\Fonts\msyhbd.ttc" if bold else r"C:\Windows\Fonts\msyh.ttc",
        r"C:\Windows\Fonts\msyh.ttc",
        r"C:\Windows\Fonts\simhei.ttf",
        r"C:\Windows\Fonts\segoeui.ttf",
    ]
    for p in candidates:
        try:
            return ImageFont.truetype(p, size)
        except OSError:
            continue
    return ImageFont.load_default()


F20 = font(20, True)
F16 = font(16, True)
F14 = font(14)
F13 = font(13)
F12 = font(12)


def round_rect(draw, xy, r, fill, outline=None, width=1):
    draw.rounded_rectangle(xy, radius=r, fill=fill, outline=outline, width=width)


def badge(draw, x, y, n: str):
    r = 12
    draw.ellipse((x - r, y - r, x + r, y + r), fill=ACCENT)
    tw = draw.textlength(n, font=F12)
    draw.text((x - tw / 2, y - 7), n, fill=(255, 255, 255), font=F12)


def button(draw, xy, text, fill):
    round_rect(draw, xy, 8, fill)
    x0, y0, x1, y1 = xy
    tw = draw.textlength(text, font=F12)
    th = 14
    draw.text(((x0 + x1 - tw) / 2, (y0 + y1 - th) / 2), text, fill=(255, 255, 255), font=F12)


def save(img: Image.Image, name: str):
    path = OUT / name
    img.save(path, "PNG", optimize=True)
    print("wrote", path)


def make_download():
    img = Image.new("RGB", (W, 360), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "第一步：下载软件（拦截时点「保留 / 保持」）", fill=NAVY, font=F20)
    d.text((40, 56), "从 GitHub Release 下载 WinToolbox-v*.exe。未签名文件常被浏览器拦截。", fill=MUTED, font=F13)
    round_rect(d, (40, 96, 920, 196), 14, CARD, LINE)
    round_rect(d, (60, 120, 900, 172), 10, (27, 27, 31))
    d.text((80, 138), "WinToolbox-v1.2.0.exe · 已阻止", fill=(255, 255, 255), font=F14)
    button(d, (700, 130, 788, 162), "保留", BLUE)
    badge(d, 690, 146, "1")
    round_rect(d, (40, 220, 920, 320), 14, (255, 247, 237), (253, 186, 116))
    d.text((64, 240), "说明", fill=(154, 52, 18), font=F16)
    d.text((64, 270), "① 点「保留 / 保持 / Keep」。② 运行若再被 SmartScreen 拦截：更多信息 → 仍要运行。", fill=MUTED, font=F13)
    d.text((64, 294), "下载页：https://github.com/datuzi-vip/wintoolbox/releases/latest", fill=MUTED, font=F12)
    save(img, "00-download-keep.png")


def make_nav():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    round_rect(d, (40, 36, 920, 504), 18, CARD)
    round_rect(d, (40, 36, 260, 504), 18, SIDE)
    d.rectangle((40, 36, 260, 100), fill=(18, 40, 63))
    d.text((70, 58), "WinToolbox", fill=(255, 255, 255), font=F16)
    items = [
        (118, "本机概览", False),
        (154, "① 本地账户", True),
        (190, "② 远程桌面", True),
        (226, "③ 防火墙", True),
        (262, "④ 安全加固", True),
        (302, "防病毒", False),
        (338, "时间同步", False),
        (374, "电源", False),
        (410, "⑤ 系统更新", True),
        (446, "软件更新", False),
    ]
    for y, label, hi in items:
        if "安全加固" in label:
            round_rect(d, (56, y - 18, 244, y + 10), 8, BLUE)
        col = (255, 255, 255) if hi or "安全加固" in label else (215, 227, 240)
        d.text((70, y - 10), label, fill=col, font=F14)

    d.text((300, 70), "系统加固推荐入口", fill=NAVY, font=F20)
    d.text((300, 102), "按编号顺序进入对应页面，图标旁数字与正文一致。", fill=MUTED, font=F13)

    rows = [
        (148, "1", "本地账户", "改密码 · 锁定策略 · 禁用来宾 / 自动登录"),
        (216, "2", "远程桌面", "改端口 · 开启服务 · 强制 NLA"),
        (284, "3", "防火墙", "全开配置文件 · 禁 ping · 拦截高危端口"),
        (352, "4", "安全加固", "禁用 SMBv1 · 关闭 WinRM · 限制匿名枚举"),
        (420, "5", "系统更新", "可选：临时关闭 Windows Update（运维场景）"),
    ]
    for y, n, title, desc in rows:
        round_rect(d, (300, y, 860, y + 56), 12, (238, 244, 250), LINE)
        badge(d, 330, y + 28, n)
        d.text((356, y + 12), title, fill=NAVY, font=F14)
        d.text((356, y + 34), desc, fill=MUTED, font=F12)
    save(img, "01-hardening-nav.png")


def make_account():
    img = Image.new("RGB", (W, 560), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "本地账户 · 操作选项标注", fill=NAVY, font=F20)
    d.text((40, 52), "红色圆点数字对应正文步骤；优先处理 ①②，再设置 ③④，最后执行 ⑤ 改密码。", fill=MUTED, font=F12)

    round_rect(d, (40, 90, 470, 240), 14, CARD, LINE)
    d.text((60, 108), "来宾账户 / 自动登录", fill=NAVY, font=F14)
    round_rect(d, (60, 136, 150, 158), 6, (237, 242, 247))
    d.text((70, 140), "来宾=启用", fill=MUTED, font=F12)
    button(d, (60, 178, 200, 212), "禁用来宾账户", ACCENT)
    badge(d, 55, 195, "1")
    button(d, (220, 178, 360, 212), "关闭自动登录", ACCENT)
    badge(d, 215, 195, "2")

    round_rect(d, (490, 90, 920, 240), 14, CARD, LINE)
    d.text((510, 108), "密码策略", fill=NAVY, font=F14)
    d.text((510, 142), "最短长度", fill=MUTED, font=F12)
    round_rect(d, (580, 134, 660, 162), 6, (248, 250, 252), LINE)
    d.text((606, 140), "12", fill=NAVY, font=F14)
    button(d, (510, 178, 640, 212), "应用密码策略", BLUE)
    badge(d, 505, 195, "3")

    round_rect(d, (40, 260, 920, 390), 14, CARD, LINE)
    d.text((60, 280), "账户锁定策略", fill=NAVY, font=F14)
    d.text((60, 310), "阈值 10 · 锁定 30 分钟 · 复位窗口 30 分钟", fill=MUTED, font=F12)
    button(d, (60, 338, 190, 372), "一键开启锁定", GREEN)
    badge(d, 55, 355, "4")
    button(d, (210, 338, 340, 372), "一键关闭锁定", (113, 128, 150))
    button(d, (360, 338, 500, 372), "应用自定义策略", BLUE)

    round_rect(d, (40, 410, 920, 530), 14, CARD, LINE)
    d.text((60, 430), "账户操作 · 修改密码", fill=NAVY, font=F14)
    d.text((60, 462), "用户", fill=MUTED, font=F12)
    round_rect(d, (100, 452, 260, 480), 6, (248, 250, 252), LINE)
    d.text((112, 458), "Administrator", fill=MUTED, font=F12)
    button(d, (60, 490, 160, 522), "修改密码", BLUE)
    badge(d, 55, 506, "5")
    button(d, (180, 490, 290, 522), "设为管理员", ORANGE)
    save(img, "02-account-ops.png")


def make_rdp():
    img = Image.new("RGB", (W, 520), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "远程桌面 · 修改端口 / NLA", fill=NAVY, font=F20)
    d.text((40, 52), "侧栏进入「远程桌面」。保存端口会同步 WinToolbox-RDP 防火墙放行规则。", fill=MUTED, font=F12)
    round_rect(d, (40, 90, 920, 390), 16, CARD, LINE)
    d.text((64, 116), "服务状态", fill=NAVY, font=F16)
    round_rect(d, (64, 144, 134, 168), 6, (198, 246, 213))
    d.text((76, 148), "已开启", fill=(39, 103, 73), font=F12)
    d.text((150, 148), "端口 3389 → 建议改为非常用端口", fill=MUTED, font=F12)
    d.text((64, 200), "端口", fill=MUTED, font=F12)
    round_rect(d, (110, 220, 230, 254), 8, (248, 250, 252), LINE)
    d.text((140, 228), "45980", fill=NAVY, font=F14)
    badge(d, 100, 237, "1")
    button(d, (260, 220, 350, 254), "保存端口", BLUE)
    badge(d, 255, 237, "2")
    button(d, (370, 220, 440, 254), "开启", ORANGE)
    badge(d, 365, 237, "3")
    button(d, (460, 220, 550, 254), "强制 NLA", GREEN)
    badge(d, 455, 237, "4")
    d.text((64, 300), "① 填写端口 → ② 保存端口 → ③ 确认已开启 → ④ 建议强制 NLA", fill=MUTED, font=F13)
    d.text((64, 340), "客户端使用 IP:新端口，例如 192.168.1.10:45980。云主机请同步放行安全组。", fill=MUTED, font=F12)
    round_rect(d, (40, 416, 920, 488), 14, (255, 247, 237), (254, 215, 170))
    d.text((64, 440), "连接提示", fill=(154, 52, 18), font=F14)
    d.text((64, 466), "改端口后务必用新端口连接，并检查云安全组是否仍放行 3389。", fill=MUTED, font=F12)
    save(img, "03-rdp-port.png")


def make_update():
    img = Image.new("RGB", (W, 480), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "系统更新 · 关闭 / 恢复", fill=NAVY, font=F20)
    d.text((40, 52), "侧栏「系统更新」。恢复时还原关闭前的策略与服务配置（非整机/云盘快照）。", fill=MUTED, font=F12)
    round_rect(d, (40, 90, 920, 310), 16, CARD, LINE)
    d.text((64, 120), "Windows Update", fill=NAVY, font=F16)
    round_rect(d, (64, 152, 128, 178), 6, (198, 246, 213))
    d.text((76, 156), "运行中", fill=(39, 103, 73), font=F12)
    d.text((64, 200), "明细示例：NoAutoUpdate=0 · wuauserv=auto · UsoSvc=auto", fill=MUTED, font=F12)
    button(d, (64, 240, 184, 280), "关闭更新", RED)
    badge(d, 58, 260, "1")
    button(d, (210, 240, 330, 280), "恢复更新", GREEN)
    badge(d, 204, 260, "2")
    round_rect(d, (40, 340, 920, 440), 14, (255, 245, 245), (254, 178, 178))
    d.text((64, 366), "注意", fill=(155, 44, 44), font=F14)
    d.text((64, 396), "① 适用于临时运维窗口。② 域策略可能覆盖。③ 状态未知时请先刷新。", fill=MUTED, font=F12)
    save(img, "04-windows-update.png")


def make_harden():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "防火墙 + 安全加固 · 关键选项", fill=NAVY, font=F20)
    d.text((40, 52), "先开防火墙与拦截高危端口，再执行 SMBv1 / WinRM / 匿名枚举加固。", fill=MUTED, font=F12)

    round_rect(d, (40, 88, 470, 478), 16, CARD, LINE)
    d.text((64, 112), "防火墙", fill=NAVY, font=F16)
    button(d, (64, 150, 210, 184), "一键开启全部", BLUE)
    badge(d, 58, 167, "1")
    button(d, (230, 150, 340, 184), "禁 ping", ORANGE)
    badge(d, 224, 167, "2")
    d.text((64, 220), "高危端口预设", fill=NAVY, font=F14)
    d.text((64, 248), "135 / 139 / 445 / 5985 / 5986（不含 3389）", fill=MUTED, font=F12)
    button(d, (64, 274, 210, 308), "拦截高危端口", ORANGE)
    badge(d, 58, 291, "3")
    d.text((64, 340), "用于降低 SMB / RPC / WinRM 入站暴露。", fill=MUTED, font=F12)
    d.text((64, 366), "若需文件共享或远程管理，请勿盲目拦截。", fill=MUTED, font=F12)

    round_rect(d, (490, 88, 920, 478), 16, CARD, LINE)
    d.text((514, 112), "安全加固", fill=NAVY, font=F16)
    d.text((514, 156), "SMBv1", fill=MUTED, font=F12)
    button(d, (514, 178, 640, 212), "禁用 SMBv1", ORANGE)
    badge(d, 508, 195, "4")
    d.text((514, 246), "WinRM", fill=MUTED, font=F12)
    button(d, (514, 268, 690, 302), "关闭并拦截 WinRM", ORANGE)
    badge(d, 508, 285, "5")
    d.text((514, 336), "匿名枚举", fill=MUTED, font=F12)
    button(d, (514, 358, 650, 392), "禁匿名枚举", ORANGE)
    badge(d, 508, 375, "6")
    d.text((514, 430), "域环境可能被组策略覆盖；操作前确认业务依赖。", fill=MUTED, font=F12)
    save(img, "05-firewall-harden.png")


def make_password():
    img = Image.new("RGB", (W, 420), BG)
    d = ImageDraw.Draw(img)
    d.text((40, 24), "修改本地账户密码", fill=NAVY, font=F20)
    d.text((40, 52), "侧栏「本地账户」→ 账户操作。需管理员权限。", fill=MUTED, font=F12)
    round_rect(d, (40, 90, 920, 310), 16, CARD, LINE)
    d.text((64, 118), "账户操作", fill=NAVY, font=F16)
    d.text((64, 160), "① 选择用户", fill=MUTED, font=F12)
    round_rect(d, (160, 148, 360, 182), 8, (248, 250, 252), LINE)
    d.text((176, 156), "Administrator", fill=NAVY, font=F14)
    badge(d, 150, 165, "1")
    d.text((64, 220), "② 新密码", fill=MUTED, font=F12)
    round_rect(d, (160, 208, 360, 242), 8, (248, 250, 252), LINE)
    d.text((176, 216), "••••••••••••", fill=MUTED, font=F14)
    badge(d, 150, 225, "2")
    d.text((400, 220), "③ 确认密码", fill=MUTED, font=F12)
    round_rect(d, (500, 208, 700, 242), 8, (248, 250, 252), LINE)
    d.text((516, 216), "••••••••••••", fill=MUTED, font=F14)
    badge(d, 490, 225, "3")
    button(d, (64, 260, 184, 300), "修改密码", BLUE)
    badge(d, 58, 280, "4")
    round_rect(d, (40, 334, 920, 394), 12, (240, 255, 244), (154, 230, 180))
    d.text((64, 358), "建议：长度 ≥ 12，含大小写/数字/符号；改完后用新密码登录验证。", fill=MUTED, font=F12)
    save(img, "06-change-password.png")


if __name__ == "__main__":
    make_download()
    make_nav()
    make_account()
    make_rdp()
    make_update()
    make_harden()
    make_password()
    print("done")
