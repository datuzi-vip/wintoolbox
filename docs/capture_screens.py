# -*- coding: utf-8 -*-
"""Capture real WinToolbox UI screenshots for docs (mocked Go bindings via Playwright)."""
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

L_ACCOUNT = "\u672c\u5730\u8d26\u6237"
L_RDP = "\u8fdc\u7a0b\u684c\u9762"
L_FW = "\u9632\u706b\u5899"
L_HARDEN = "\u5b89\u5168\u52a0\u56fa"
L_UPDATE = "\u7cfb\u7edf\u66f4\u65b0"
B_DISABLE_GUEST = "\u7981\u7528\u6765\u5bbe\u8d26\u6237"
B_DISABLE_AUTO = "\u5173\u95ed\u81ea\u52a8\u767b\u5f55"
B_PWD_POLICY = "\u5e94\u7528\u5bc6\u7801\u7b56\u7565"
B_LOCKOUT_ON = "\u4e00\u952e\u5f00\u542f\u9501\u5b9a"
B_CHANGE_PWD = "\u4fee\u6539\u5bc6\u7801"
B_SAVE_PORT = "\u4fdd\u5b58\u7aef\u53e3"
B_CLOSE = "\u5173\u95ed"
B_OPEN = "\u5f00\u542f"
B_NLA = "\u5f3a\u5236 NLA"
B_FW_ON = "\u5168\u90e8\u5f00\u542f"
B_PING = "\u4e00\u952e\u7981 ping"
B_RISK = "\u62e6\u9ad8\u5371\u5165\u7ad9\u7aef\u53e3"
B_SMB1 = "\u7981\u7528 SMBv1"
B_WINRM = "\u5173\u95ed\u5e76\u62e6\u622a WinRM"
B_ANON = "\u7981\u533f\u540d\u679a\u4e3e"
B_UPD_OFF = "\u5173\u95ed\u66f4\u65b0"
B_UPD_ON = "\u6062\u590d\u66f4\u65b0"


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


F12 = font(12, True)
ACCENT = (232, 93, 76)

MOCK_JS = r"""
(() => {
  const status = {
    overview: {
      hostname: 'WIN-SERVER01',
      osName: 'Windows Server 2022 Datacenter',
      osBuild: '20348.3807',
      arch: 'x64',
      manufacturer: 'Dell Inc.',
      model: 'PowerEdge R740',
      board: '0H21HH',
      bios: '2.18.1',
      cpu: 'Intel Xeon Silver 4210',
      cpuCores: 20,
      memoryTotalGB: 64,
      memoryAvailGB: 41.2,
      memoryModules: '4 x 16GB DDR4',
      resolution: '1920x1080',
      ips: ['10.0.0.12'],
      disks: ['C: 120GB free / 500GB'],
      physicalDisks: ['Samsung SSD 980 1TB'],
      gpus: ['Microsoft Basic Display Adapter'],
      activated: true,
      activationStatus: 'Licensed'
    },
    accounts: [
      { name: 'Administrator', enabled: true, enabledUnknown: false, admin: true, adminUnknown: false, current: true },
      { name: 'ops', enabled: true, enabledUnknown: false, admin: false, adminUnknown: false, current: false }
    ],
    rdpEnabled: true,
    rdpPort: 3389,
    rdpAvailable: true,
    rdpNLA: false,
    rdpNLAUnknown: false,
    updateDisabled: false,
    updateUnknown: false,
    updateDetail: 'NoAutoUpdate=0 · wuauserv=auto · UsoSvc=auto · DoSvc=auto',
    firewallSummary: 'all-on',
    firewallDomain: '\u5f00',
    firewallPrivate: '\u5f00',
    firewallPublic: '\u5f00',
    firewallAllOn: true,
    firewallAllOff: false,
    pingBlocked: false,
    pingIPv4Blocked: false,
    pingIPv6Blocked: false,
    pingState: 'enabled',
    riskPortsBlocked: false,
    riskPortsPartial: false,
    riskPortsUnknown: false,
    riskPortsDetail: '\u9ad8\u5371\u7aef\u53e3\u9884\u8bbe\u672a\u542f\u7528',
    defenderDisabled: false,
    defenderUnknown: false,
    defenderDetail: 'DisableRealtimeMonitoring=false',
    timeText: '2026-09-11 09:30:00  ·  China Standard Time  ·  NTP time.windows.com',
    timeZone: 'China Standard Time',
    timeUnknown: false,
    timeZones: [{ id: 'China Standard Time', label: '(UTC+08:00) Beijing' }],
    ntpServer: 'time.windows.com',
    warnings: [],
    lockoutDisabled: true,
    lockoutUnknown: false,
    lockoutDetail: 'Threshold=0',
    lockoutThreshold: 0,
    lockoutDuration: 30,
    lockoutWindow: 30,
    guestExists: true,
    guestEnabled: true,
    guestUnknown: false,
    guestDetail: 'Guest enabled',
    autoLogonEnabled: true,
    autoLogonUnknown: false,
    autoLogonDetail: 'AutoAdminLogon=1',
    passwordMinLength: 8,
    passwordComplexity: false,
    passwordComplexityUnknown: false,
    passwordUnknown: false,
    passwordPolicyDetail: 'MinLen=8',
    smb1Disabled: false,
    smb1Unknown: false,
    smb1Detail: 'SMBv1 enabled',
    winrmHardened: false,
    winrmUnknown: false,
    winrmDetail: 'WinRM not hardened',
    anonymousOK: false,
    anonymousUnknown: false,
    anonymousDetail: 'Anonymous enum allowed'
  };
  const appInfo = { name: 'WinToolbox', version: 'v1.2.0' };
  const rules = [{ name: 'WinToolbox-Allow-8080', port: '8080', enabled: true }];
  const history = [{ kind: 'mru', host: '10.0.0.8', username: 'Administrator', source: 'MRU', detail: '', sid: '' }];
  const upd = { currentVersion: 'v1.2.0', latestVersion: 'v1.2.0', hasUpdate: false, assetName: '', assetURL: '', assetSize: 0, assetSHA256: '', notes: '', downloaded: false, downloadPath: '', verified: false, error: '' };
  const ok = (v) => Promise.resolve(v);
  const noop = () => ok(null);
  const app = {
    GetAppInfo: () => ok(appInfo),
    GetStatus: () => ok(status),
    GetOverviewDetail: () => ok({ memoryModules: status.overview.memoryModules, physicalDisks: status.overview.physicalDisks, gpus: status.overview.gpus, activated: true, activationStatus: 'Licensed' }),
    GetTimeZones: () => ok(status.timeZones),
    GetFirewallRules: () => ok(rules),
    GetRdpHistory: () => ok(history),
    CheckAppUpdate: () => ok(upd),
    DownloadAppUpdate: () => ok(upd),
    GetAppUpdateInfo: () => ok(upd),
    ApplyAppUpdate: noop,
    ChangeAccountPassword: noop, SetAccountEnabled: noop, SetAccountAdmin: noop,
    DisableAccountLockout: noop, EnableAccountLockout: noop, SetAccountLockoutPolicy: noop,
    DisableGuestAccount: noop, DisableAutoLogon: noop, EnablePasswordPolicy: noop,
    ChangeRdpPort: noop, ToggleRdp: noop, SetRdpNLA: noop,
    ClearRdpHistory: noop, ClearRdpHistoryByKind: noop, DeleteRdpHistoryEntry: noop,
    AllowFirewallPort: noop, RemoveFirewallPort: noop, ClearFirewallAllowRules: noop,
    SetFirewallEnabled: noop, DisablePing: noop, EnablePing: noop,
    BlockRiskPorts: noop, UnblockRiskPorts: noop,
    DisableSMBv1: noop, HardenWinRM: noop, RestrictAnonymous: noop,
    ApplyTimeZone: noop, SaveNTPServer: noop, SyncNTP: noop, TestNTPServer: () => ok('ok'),
    LockPC: noop, RestartPC: noop, ShutdownPC: noop, AbortPower: noop,
    DisableUpdate: noop, EnableUpdate: noop, DisableDefender: noop, EnableDefender: noop
  };
  window.go = { ui: { App: app } };
})();
"""


def badge(draw, x, y, n: str):
    r = 14
    draw.ellipse((x - r, y - r, x + r, y + r), fill=ACCENT, outline=(255, 255, 255), width=2)
    tw = draw.textlength(n, font=F12)
    draw.text((x - tw / 2, y - 8), n, fill=(255, 255, 255), font=F12)


SCALE = 1.0


def annotate(png_path: Path, marks):
    img = Image.open(png_path).convert("RGBA")
    overlay = Image.new("RGBA", img.size, (0, 0, 0, 0))
    d = ImageDraw.Draw(overlay)
    for x, y, n in marks:
        badge(d, x * SCALE, y * SCALE, n)
    Image.alpha_composite(img, overlay).convert("RGB").save(png_path, "PNG", optimize=True)


def wait_ready(page):
    page.wait_for_selector(".side-menu", timeout=30000)
    page.wait_for_timeout(800)


def click_menu(page, title: str):
    page.locator(".side-menu .el-menu-item", has_text=title).first.click()
    page.wait_for_timeout(800)


def box_left(page, locator, scroll: bool = False):
    """Badge anchor: left-center of element (viewport coords)."""
    if locator.count() == 0:
        return None
    el = locator.first
    if scroll:
        try:
            el.scroll_into_view_if_needed(timeout=5000)
        except Exception:
            return None
        page.wait_for_timeout(120)
    b = el.bounding_box()
    if not b:
        return None
    if b["y"] + b["height"] < 8 or b["y"] > 980:
        return None
    return (b["x"] - 14, b["y"] + b["height"] / 2)


def mark_buttons(page, pairs, scroll: bool = False):
    marks = []
    for text, n in pairs:
        loc = page.get_by_role("button", name=text)
        if loc.count() == 0:
            loc = page.locator("button", has_text=text)
        c = box_left(page, loc, scroll=scroll)
        if c:
            marks.append((c[0], c[1], n))
    return marks


def shot(page, path: Path):
    page.keyboard.press("Escape")
    page.wait_for_timeout(150)
    page.locator(".app-shell").first.screenshot(path=str(path))


def capture_nav(page):
    path = OUT / "01-hardening-nav.png"
    shot(page, path)
    marks = []
    for title, n in ((L_ACCOUNT, "1"), (L_RDP, "2"), (L_FW, "3"), (L_HARDEN, "4"), (L_UPDATE, "5")):
        c = box_left(page, page.locator(".side-menu .el-menu-item", has_text=title))
        if c:
            marks.append((c[0], c[1], n))
    annotate(path, marks)
    print("wrote", path)


def capture_account(page):
    click_menu(page, L_ACCOUNT)
    page.locator(".page-title").first.scroll_into_view_if_needed()
    # Bring lockout into view so ①–④ fit one frame.
    loc = page.get_by_role("button", name=B_LOCKOUT_ON)
    if loc.count():
        loc.first.scroll_into_view_if_needed()
        page.wait_for_timeout(200)
    page.locator(".page-title").first.scroll_into_view_if_needed()
    page.wait_for_timeout(200)
    path = OUT / "02-account-ops.png"
    # Annotate only controls visible in the upper page (password change has its own shot).
    marks = mark_buttons(
        page,
        [
            (B_DISABLE_GUEST, "1"),
            (B_DISABLE_AUTO, "2"),
            (B_PWD_POLICY, "3"),
            (B_LOCKOUT_ON, "4"),
        ],
    )
    shot(page, path)
    annotate(path, marks)
    print("wrote", path)

    path6 = OUT / "06-change-password.png"
    btn = page.get_by_role("button", name=B_CHANGE_PWD)
    if btn.count():
        btn.first.scroll_into_view_if_needed()
        page.wait_for_timeout(250)
    marks6 = []
    c = box_left(page, page.locator(".el-form-item", has_text="\u7528\u6237").locator(".el-select").first)
    if not c:
        c = box_left(page, page.locator(".el-select").first)
    if c:
        marks6.append((c[0], c[1], "1"))
    pwds = page.locator("input[type=password]")
    if pwds.count() >= 1:
        c = box_left(page, pwds.nth(0))
        if c:
            marks6.append((c[0], c[1], "2"))
    if pwds.count() >= 2:
        c = box_left(page, pwds.nth(1))
        if c:
            marks6.append((c[0], c[1], "3"))
    marks6 += mark_buttons(page, [(B_CHANGE_PWD, "4")])
    shot(page, path6)
    annotate(path6, marks6)
    print("wrote", path6)


def capture_rdp(page):
    click_menu(page, L_RDP)
    page.wait_for_timeout(400)
    path = OUT / "03-rdp-port.png"
    marks = []
    c = box_left(page, page.locator(".el-input-number").first)
    if c:
        marks.append((c[0], c[1], "1"))
    marks += mark_buttons(page, [(B_SAVE_PORT, "2")])
    c = box_left(page, page.get_by_role("button", name=B_CLOSE))
    if not c:
        c = box_left(page, page.get_by_role("button", name=B_OPEN))
    if c:
        marks.append((c[0], c[1], "3"))
    marks += mark_buttons(page, [(B_NLA, "4")])
    shot(page, path)
    annotate(path, marks)
    print("wrote", path, "size", path.stat().st_size)


def capture_firewall_harden(page):
    click_menu(page, L_FW)
    risk = page.get_by_role("button", name=B_RISK)
    if risk.count():
        risk.first.scroll_into_view_if_needed()
        page.wait_for_timeout(200)
    marks = mark_buttons(page, [(B_FW_ON, "1"), (B_PING, "2"), (B_RISK, "3")], scroll=False)
    path = OUT / "05-firewall-harden.png"
    shot(page, path)
    annotate(path, marks)
    print("wrote", path)

    click_menu(page, L_HARDEN)
    page.locator(".page-title").first.scroll_into_view_if_needed()
    page.wait_for_timeout(150)
    marks_h = mark_buttons(page, [(B_SMB1, "4"), (B_WINRM, "5"), (B_ANON, "6")], scroll=False)
    # If anon off-screen, scroll once then remeasure.
    if len(marks_h) < 3:
        anon = page.get_by_role("button", name=B_ANON)
        if anon.count():
            anon.first.scroll_into_view_if_needed()
            page.wait_for_timeout(150)
        marks_h = mark_buttons(page, [(B_SMB1, "4"), (B_WINRM, "5"), (B_ANON, "6")], scroll=False)
    path_h = OUT / "05b-harden-ops.png"
    shot(page, path_h)
    annotate(path_h, marks_h)
    print("wrote", path_h)


def capture_update(page):
    click_menu(page, L_UPDATE)
    path = OUT / "04-windows-update.png"
    marks = mark_buttons(page, [(B_UPD_OFF, "1"), (B_UPD_ON, "2")])
    shot(page, path)
    annotate(path, marks)
    print("wrote", path)


def make_download_mock():
    from gen_images import make_download

    make_download()


def main():
    print("labels ok:", L_ACCOUNT, B_DISABLE_GUEST)
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
            page = browser.new_context(viewport={"width": 1280, "height": 1000}, device_scale_factor=1).new_page()
            page.add_init_script(MOCK_JS)
            page.goto(BASE, wait_until="networkidle", timeout=60000)
            wait_ready(page)
            page.keyboard.press("Escape")
            page.add_style_tag(content=".op-log, .wt-log { max-height: 72px !important; }")
            capture_nav(page)
            capture_account(page)
            capture_rdp(page)
            capture_update(page)
            capture_firewall_harden(page)
            browser.close()
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
    print("done")


if __name__ == "__main__":
    main()
