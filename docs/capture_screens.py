# -*- coding: utf-8 -*-
"""Capture HD annotated screenshots for all WinToolbox feature docs."""
from __future__ import annotations

import subprocess
import sys
import time
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[1]
FRONTEND = ROOT / "frontend"
OUT = Path(__file__).resolve().parent / "images"
OUT.mkdir(parents=True, exist_ok=True)
PORT = 5179
BASE = f"http://127.0.0.1:{PORT}/"
SCALE = 2  # device_scale_factor for crisp docs images
VIEW_W, VIEW_H = 1360, 1100

# Menu titles
M_OVERVIEW = "\u672c\u673a\u6982\u89c8"
M_ACCOUNT = "\u672c\u5730\u8d26\u6237"
M_RDP = "\u8fdc\u7a0b\u684c\u9762"
M_FW = "\u9632\u706b\u5899"
M_HARDEN = "\u5b89\u5168\u52a0\u56fa"
M_DEF = "\u9632\u75c5\u6bd2"
M_TIME = "\u65f6\u95f4\u540c\u6b65"
M_POWER = "\u7535\u6e90"
M_UPD = "\u7cfb\u7edf\u66f4\u65b0"
M_SELF = "\u8f6f\u4ef6\u66f4\u65b0"

# Buttons / labels
B_DISABLE_GUEST = "\u7981\u7528\u6765\u5bbe\u8d26\u6237"
B_DISABLE_AUTO = "\u5173\u95ed\u81ea\u52a8\u767b\u5f55"
B_PWD_POLICY = "\u5e94\u7528\u5bc6\u7801\u7b56\u7565"
B_LOCKOUT_ON = "\u4e00\u952e\u5f00\u542f\u9501\u5b9a"
B_CHANGE_PWD = "\u4fee\u6539\u5bc6\u7801"
B_ENABLE_USER = "\u542f\u7528"
B_DISABLE_USER = "\u7981\u7528"
B_SET_ADMIN = "\u8bbe\u4e3a\u7ba1\u7406\u5458"
B_SAVE_PORT = "\u4fdd\u5b58\u7aef\u53e3"
B_CLOSE = "\u5173\u95ed"
B_OPEN = "\u5f00\u542f"
B_NLA = "\u5f3a\u5236 NLA"
B_FW_ON = "\u5168\u90e8\u5f00\u542f"
B_FW_OFF = "\u5168\u90e8\u5173\u95ed"
B_ALLOW = "\u653e\u884c"
B_PING = "\u4e00\u952e\u7981 ping"
B_RISK = "\u62e6\u9ad8\u5371\u5165\u7ad9\u7aef\u53e3"
B_SMB1 = "\u7981\u7528 SMBv1"
B_WINRM = "\u5173\u95ed\u5e76\u62e6\u622a WinRM"
B_ANON = "\u7981\u533f\u540d\u679a\u4e3e"
B_DEF_OFF = "\u5173\u95ed\u5b9e\u65f6\u9632\u62a4"
B_DEF_ON = "\u6062\u590d\u5b9e\u65f6\u9632\u62a4"
B_TZ = "\u5e94\u7528\u65f6\u533a"
B_NTP_SAVE = "\u4fdd\u5b58 NTP"
B_NTP_SYNC = "\u7acb\u5373\u540c\u6b65"
B_NTP_TEST = "\u6d4b\u8bd5 NTP"
B_LOCK = "\u9501\u5b9a"
B_RESTART = "\u91cd\u542f"
B_SHUTDOWN = "\u5173\u673a"
B_ABORT = "\u53d6\u6d88\u5173\u673a/\u91cd\u542f"
B_UPD_OFF = "\u5173\u95ed\u66f4\u65b0"
B_UPD_ON = "\u6062\u590d\u66f4\u65b0"
B_CHECK = "\u68c0\u67e5\u66f4\u65b0"
B_DL = "\u4e0b\u8f7d\u66f4\u65b0"
B_INSTALL = "\u5b89\u88c5\u5e76\u91cd\u542f"


def font(size: int, bold: bool = False):
    paths = (
        [r"C:\Windows\Fonts\msyhbd.ttc", r"C:\Windows\Fonts\msyh.ttc"]
        if bold
        else [r"C:\Windows\Fonts\msyh.ttc", r"C:\Windows\Fonts\simhei.ttf"]
    )
    for p in paths:
        try:
            return ImageFont.truetype(p, size)
        except OSError:
            pass
    return ImageFont.load_default()


F_BADGE = font(18, True)
ACCENT = (232, 93, 76)

MOCK_JS = r"""
(() => {
  const status = {
    overview: {
      hostname: 'WIN-SERVER01', osName: 'Windows Server 2022 Datacenter', osBuild: '20348.3807', arch: 'x64',
      manufacturer: 'Dell Inc.', model: 'PowerEdge R740', board: '0H21HH', bios: '2.18.1',
      cpu: 'Intel Xeon Silver 4210', cpuCores: 20, memoryTotalGB: 64, memoryAvailGB: 41.2,
      memoryModules: '4 x 16GB DDR4', resolution: '1920x1080', ips: ['10.0.0.12'],
      disks: ['C: 120GB free / 500GB'], physicalDisks: ['Samsung SSD 980 1TB'],
      gpus: ['Microsoft Basic Display Adapter'], activated: true, activationStatus: 'Licensed'
    },
    accounts: [
      { name: 'Administrator', enabled: true, enabledUnknown: false, admin: true, adminUnknown: false, current: true },
      { name: 'ops', enabled: true, enabledUnknown: false, admin: false, adminUnknown: false, current: false }
    ],
    rdpEnabled: true, rdpPort: 3389, rdpAvailable: true, rdpNLA: false, rdpNLAUnknown: false,
    updateDisabled: false, updateUnknown: false,
    updateDetail: 'NoAutoUpdate=0 · wuauserv=auto · UsoSvc=auto · DoSvc=auto',
    firewallSummary: 'all-on', firewallDomain: '\u5f00', firewallPrivate: '\u5f00', firewallPublic: '\u5f00',
    firewallAllOn: true, firewallAllOff: false,
    pingBlocked: false, pingIPv4Blocked: false, pingIPv6Blocked: false, pingState: 'enabled',
    riskPortsBlocked: false, riskPortsPartial: false, riskPortsUnknown: false,
    riskPortsDetail: '\u9ad8\u5371\u7aef\u53e3\u9884\u8bbe\u672a\u542f\u7528',
    defenderDisabled: false, defenderUnknown: false, defenderDetail: 'DisableRealtimeMonitoring=false',
    timeText: '2026-09-11 12:00:00  ·  China Standard Time  ·  NTP time.windows.com',
    timeZone: 'China Standard Time', timeUnknown: false,
    timeZones: [{ id: 'China Standard Time', label: '(UTC+08:00) Beijing' }],
    ntpServer: 'time.windows.com', warnings: [],
    lockoutDisabled: true, lockoutUnknown: false, lockoutDetail: 'Threshold=0',
    lockoutThreshold: 0, lockoutDuration: 30, lockoutWindow: 30,
    guestExists: true, guestEnabled: true, guestUnknown: false, guestDetail: 'Guest enabled',
    autoLogonEnabled: true, autoLogonUnknown: false, autoLogonDetail: 'AutoAdminLogon=1',
    passwordMinLength: 8, passwordComplexity: false, passwordComplexityUnknown: false,
    passwordUnknown: false, passwordPolicyDetail: 'MinLen=8',
    smb1Disabled: false, smb1Unknown: false, smb1Detail: 'SMBv1 enabled',
    winrmHardened: false, winrmUnknown: false, winrmDetail: 'WinRM not hardened',
    anonymousOK: false, anonymousUnknown: false, anonymousDetail: 'Anonymous enum allowed'
  };
  const appInfo = { name: 'WinToolbox', version: 'v1.2.0' };
  const rules = [{ name: 'WinToolbox-Allow-8080', port: '8080', enabled: true }];
  const history = [{ kind: 'mru', host: '10.0.0.8', username: 'Administrator', source: 'MRU', detail: '', sid: '' }];
  const upd = { currentVersion: 'v1.2.0', latestVersion: 'v1.2.0', hasUpdate: false, assetName: '', assetURL: '', assetSize: 0, assetSHA256: '', notes: '', downloaded: false, downloadPath: '', verified: false, error: '' };
  const ok = (v) => Promise.resolve(v);
  const noop = () => ok(null);
  const app = {
    GetAppInfo: () => ok(appInfo), GetStatus: () => ok(status),
    GetOverviewDetail: () => ok({ memoryModules: status.overview.memoryModules, physicalDisks: status.overview.physicalDisks, gpus: status.overview.gpus, activated: true, activationStatus: 'Licensed' }),
    GetTimeZones: () => ok(status.timeZones), GetFirewallRules: () => ok(rules), GetRdpHistory: () => ok(history),
    CheckAppUpdate: () => ok(upd), DownloadAppUpdate: () => ok(upd), GetAppUpdateInfo: () => ok(upd), ApplyAppUpdate: noop,
    ChangeAccountPassword: noop, SetAccountEnabled: noop, SetAccountAdmin: noop,
    DisableAccountLockout: noop, EnableAccountLockout: noop, SetAccountLockoutPolicy: noop,
    DisableGuestAccount: noop, DisableAutoLogon: noop, EnablePasswordPolicy: noop,
    ChangeRdpPort: noop, ToggleRdp: noop, SetRdpNLA: noop,
    ClearRdpHistory: noop, ClearRdpHistoryByKind: noop, DeleteRdpHistoryEntry: noop,
    AllowFirewallPort: noop, RemoveFirewallPort: noop, ClearFirewallAllowRules: noop,
    SetFirewallEnabled: noop, DisablePing: noop, EnablePing: noop, BlockRiskPorts: noop, UnblockRiskPorts: noop,
    DisableSMBv1: noop, HardenWinRM: noop, RestrictAnonymous: noop,
    ApplyTimeZone: noop, SaveNTPServer: noop, SyncNTP: noop, TestNTPServer: () => ok('ok'),
    LockPC: noop, RestartPC: noop, ShutdownPC: noop, AbortPower: noop,
    DisableUpdate: noop, EnableUpdate: noop, DisableDefender: noop, EnableDefender: noop
  };
  window.go = { ui: { App: app } };
})();
"""


def badge(draw, x, y, n: str):
    r = 18
    # white ring + filled circle for contrast on any background
    draw.ellipse((x - r - 2, y - r - 2, x + r + 2, y + r + 2), fill=(255, 255, 255))
    draw.ellipse((x - r, y - r, x + r, y + r), fill=ACCENT)
    tw = draw.textlength(n, font=F_BADGE)
    draw.text((x - tw / 2, y - 11), n, fill=(255, 255, 255), font=F_BADGE)


def annotate(png_path: Path, marks):
    """marks are CSS viewport coords; screenshot is device pixels (SCALE)."""
    img = Image.open(png_path).convert("RGBA")
    overlay = Image.new("RGBA", img.size, (0, 0, 0, 0))
    d = ImageDraw.Draw(overlay)
    for x, y, n in marks:
        badge(d, x * SCALE, y * SCALE, n)
    Image.alpha_composite(img, overlay).convert("RGB").save(png_path, "PNG", optimize=True)


def wait_ready(page):
    page.wait_for_selector(".side-menu", timeout=30000)
    page.wait_for_timeout(700)


def click_menu(page, title: str):
    item = page.locator(".side-menu .el-menu-item", has_text=title).first
    item.scroll_into_view_if_needed(timeout=5000)
    item.click()
    page.wait_for_timeout(800)
    page.keyboard.press("Escape")
    page.wait_for_timeout(120)
    # keep main content scrolled to top for consistent framing
    page.evaluate(
        """() => {
          const main = document.querySelector('.app-main');
          if (main) main.scrollTop = 0;
        }"""
    )
    page.wait_for_timeout(80)


def box_mark(page, locator, scroll: bool = False):
    """Return CSS coords for a badge on the control's top-left (avoids overlapping the previous button)."""
    if locator.count() == 0:
        return None
    el = locator.first
    if scroll:
        try:
            el.scroll_into_view_if_needed(timeout=5000)
        except Exception:
            return None
        page.wait_for_timeout(100)
    b = el.bounding_box()
    if not b:
        return None
    if b["y"] + b["height"] < 4 or b["y"] > VIEW_H - 4:
        return None
    return (b["x"] + 14, max(14, b["y"] + 12))


def btn(page, name: str):
    # exact accessible name to avoid substring collisions
    loc = page.get_by_role("button", name=name, exact=True)
    if loc.count() == 0:
        loc = page.locator("button.el-button").filter(has_text=name)
    return loc


def marks_buttons(page, pairs, scroll: bool = False):
    out = []
    for text, n in pairs:
        c = box_mark(page, btn(page, text), scroll=scroll)
        if c:
            out.append((c[0], c[1], n))
    return out


def shot(page, path: Path):
    page.keyboard.press("Escape")
    page.wait_for_timeout(150)
    page.wait_for_selector(".app-shell", state="visible", timeout=15000)
    # Viewport capture (device pixels via SCALE) — avoids element-screenshot timeouts
    page.screenshot(path=str(path), type="png", full_page=False, scale="device", timeout=60000)


def capture(page, menu: str, filename: str, pairs, prepare=None):
    click_menu(page, menu)
    if prepare:
        prepare(page)
    page.wait_for_timeout(150)
    marks = marks_buttons(page, pairs, scroll=False)
    path = OUT / filename
    shot(page, path)
    annotate(path, marks)
    print("wrote", path, path.stat().st_size)
    return path


def make_download_mock():
    """Simulated browser keep dialog — not real UI."""
    from gen_images import make_download

    make_download()
    src = OUT / "00-download-keep.png"
    dst = OUT / "00-download.png"
    if src.exists():
        Image.open(src).convert("RGB").save(dst, "PNG", optimize=True)
        print("wrote", dst)


def prep_scroll_risk(page):
    loc = btn(page, B_RISK)
    if loc.count():
        loc.first.scroll_into_view_if_needed()
        page.wait_for_timeout(150)


def prep_scroll_pwd(page):
    loc = btn(page, B_CHANGE_PWD)
    if loc.count():
        loc.first.scroll_into_view_if_needed()
        page.wait_for_timeout(150)


def prep_scroll_anon(page):
    # Prefer framing from first harden action; then ensure later ones visible if needed
    loc = btn(page, B_SMB1)
    if loc.count():
        loc.first.scroll_into_view_if_needed()
        page.wait_for_timeout(120)


def capture_nav(page):
    click_menu(page, M_OVERVIEW)
    path = OUT / "90-hardening-nav.png"
    shot(page, path)
    marks = []
    for title, n in (
        (M_ACCOUNT, "1"),
        (M_RDP, "2"),
        (M_FW, "3"),
        (M_HARDEN, "4"),
        (M_UPD, "5"),
    ):
        c = box_mark(page, page.locator(".side-menu .el-menu-item", has_text=title), scroll=False)
        if c:
            marks.append((c[0], c[1], n))
    annotate(path, marks)
    print("wrote", path)


def capture_account_full(page):
    click_menu(page, M_ACCOUNT)
    page.locator(".page-title").first.scroll_into_view_if_needed()
    page.wait_for_timeout(150)
    path = OUT / "02-local-account.png"
    marks = marks_buttons(
        page,
        [
            (B_DISABLE_GUEST, "1"),
            (B_DISABLE_AUTO, "2"),
            (B_PWD_POLICY, "3"),
            (B_LOCKOUT_ON, "4"),
        ],
        scroll=False,
    )
    shot(page, path)
    annotate(path, marks)
    print("wrote", path)

    prep_scroll_pwd(page)
    path2 = OUT / "02-local-account-password.png"
    marks2 = []
    card = page.locator(".wt-card").filter(has_text="\u8d26\u6237\u64cd\u4f5c")
    c = box_mark(page, card.locator(".el-select").first, scroll=False)
    if c:
        marks2.append((c[0], c[1], "1"))
    pwds = card.locator("input[type=password]")
    if pwds.count() >= 1:
        c = box_mark(page, pwds.nth(0), scroll=False)
        if c:
            marks2.append((c[0], c[1], "2"))
    if pwds.count() >= 2:
        c = box_mark(page, pwds.nth(1), scroll=False)
        if c:
            marks2.append((c[0], c[1], "3"))
    # buttons: 修改密码 / 启用 / 禁用 / 设为管理员 / 取消管理员
    action_btns = card.locator(".wt-actions button")
    for idx, n in ((0, "4"), (3, "5")):
        if action_btns.count() > idx:
            c = box_mark(page, action_btns.nth(idx), scroll=False)
            if c:
                marks2.append((c[0], c[1], n))
    shot(page, path2)
    annotate(path2, marks2)
    print("wrote", path2)


def capture_rdp(page):
    click_menu(page, M_RDP)
    page.wait_for_timeout(200)
    path = OUT / "03-remote-desktop.png"
    marks = []
    c = box_mark(page, page.locator(".el-input-number").first, scroll=False)
    if c:
        marks.append((c[0], c[1], "1"))
    marks += marks_buttons(page, [(B_SAVE_PORT, "2")], scroll=False)
    c = box_mark(page, btn(page, B_CLOSE), scroll=False) or box_mark(page, btn(page, B_OPEN), scroll=False)
    if c:
        marks.append((c[0], c[1], "3"))
    marks += marks_buttons(page, [(B_NLA, "4")], scroll=False)
    shot(page, path)
    annotate(path, marks)
    print("wrote", path)


def main():
    make_download_mock()
    proc = subprocess.Popen(
        f"npm run dev -- --host 127.0.0.1 --port {PORT} --strictPort",
        cwd=str(FRONTEND),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        shell=True,
    )
    try:
        t0 = time.time()
        ready = False
        while time.time() - t0 < 60:
            line = proc.stdout.readline() if proc.stdout else ""
            if line:
                sys.stdout.write(line)
                if "Local:" in line or str(PORT) in line:
                    ready = True
                    break
            if proc.poll() is not None:
                print(proc.stdout.read() if proc.stdout else "")
                raise RuntimeError("vite exited")
            time.sleep(0.05)
        if not ready:
            time.sleep(2)

        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            context = browser.new_context(
                viewport={"width": VIEW_W, "height": VIEW_H},
                device_scale_factor=SCALE,
            )
            page = context.new_page()
            page.on("pageerror", lambda err: print("pageerror:", err))
            page.add_init_script(MOCK_JS)
            page.goto(BASE, wait_until="networkidle", timeout=60000)
            wait_ready(page)
            page.add_style_tag(content=".op-log, .wt-log { max-height: 64px !important; }")

            jobs = [
                ("nav", lambda: capture_nav(page)),
                ("01", lambda: capture(page, M_OVERVIEW, "01-overview.png", [])),
                ("02", lambda: capture_account_full(page)),
                ("03", lambda: capture_rdp(page)),
                (
                    "04",
                    lambda: capture(
                        page,
                        M_FW,
                        "04-firewall.png",
                        [(B_FW_ON, "1"), (B_ALLOW, "2"), (B_PING, "3"), (B_RISK, "4")],
                        prepare=prep_scroll_risk,
                    ),
                ),
                (
                    "05",
                    lambda: capture(
                        page,
                        M_HARDEN,
                        "05-security-harden.png",
                        [(B_SMB1, "1"), (B_WINRM, "2"), (B_ANON, "3")],
                        prepare=prep_scroll_anon,
                    ),
                ),
                ("06", lambda: capture(page, M_DEF, "06-defender.png", [(B_DEF_OFF, "1"), (B_DEF_ON, "2")])),
                (
                    "07",
                    lambda: capture(
                        page,
                        M_TIME,
                        "07-time-sync.png",
                        [(B_TZ, "1"), (B_NTP_SAVE, "2"), (B_NTP_SYNC, "3"), (B_NTP_TEST, "4")],
                    ),
                ),
                (
                    "08",
                    lambda: capture(
                        page,
                        M_POWER,
                        "08-power.png",
                        [(B_LOCK, "1"), (B_RESTART, "2"), (B_SHUTDOWN, "3"), (B_ABORT, "4")],
                    ),
                ),
                ("09", lambda: capture(page, M_UPD, "09-windows-update.png", [(B_UPD_OFF, "1"), (B_UPD_ON, "2")])),
                (
                    "10",
                    lambda: capture(
                        page,
                        M_SELF,
                        "10-app-update.png",
                        [(B_CHECK, "1"), (B_DL, "2"), (B_INSTALL, "3")],
                    ),
                ),
            ]
            for name, fn in jobs:
                try:
                    fn()
                except Exception as e:
                    print(f"FAILED {name}: {e}")
                    raise
            browser.close()
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()

    keep = {
        "00-download.png",
        "00-download-keep.png",
        "01-overview.png",
        "02-local-account.png",
        "02-local-account-password.png",
        "03-remote-desktop.png",
        "04-firewall.png",
        "05-security-harden.png",
        "06-defender.png",
        "07-time-sync.png",
        "08-power.png",
        "09-windows-update.png",
        "10-app-update.png",
        "90-hardening-nav.png",
    }
    for p in OUT.glob("*.png"):
        if p.name not in keep:
            p.unlink(missing_ok=True)
            print("removed", p.name)
    print("done")


if __name__ == "__main__":
    main()
