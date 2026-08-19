package ui

import (
	"context"
	"fmt"
	"strings"

	"wintoolbox/internal/account"
	"wintoolbox/internal/defender"
	"wintoolbox/internal/firewall"
	"wintoolbox/internal/power"
	"wintoolbox/internal/rdp"
	"wintoolbox/internal/selfupdate"
	"wintoolbox/internal/update"
	"wintoolbox/internal/win/port"
	"wintoolbox/internal/wintime"
)

// App exposes WinToolbox operations to the Wails frontend (window.go.ui.App).
type App struct {
	ctx context.Context
}

// NewApp creates the bound application API.
func NewApp() *App { return &App{} }

// Startup is called by Wails when the app starts.
func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    AppName,
		Version: AppVersion,
	}
}

func (a *App) GetStatus(refresh bool) (Status, error) {
	return LoadStatus(refresh)
}

func (a *App) GetOverviewDetail() (OverviewDetail, error) {
	return LoadOverviewDetail()
}

func (a *App) GetTimeZones() ([]wintime.ZoneOption, error) {
	return LoadTimeZones()
}

func (a *App) GetFirewallRules() ([]firewall.RuleInfo, error) {
	return LoadFirewallRules()
}

func (a *App) ChangeAccountPassword(user, pass1, pass2 string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		return fmt.Errorf("请选择用户")
	}
	if pass1 == "" || pass2 == "" {
		return fmt.Errorf("密码不能为空")
	}
	if pass1 != pass2 {
		return fmt.Errorf("两次输入的密码不一致")
	}
	return account.SetPassword(user, pass1)
}

func (a *App) SetAccountEnabled(user string, enabled bool) error {
	user = strings.TrimSpace(user)
	if user == "" {
		return fmt.Errorf("请选择用户")
	}
	return account.SetEnabled(user, enabled)
}

func (a *App) SetAccountAdmin(user string, admin bool) error {
	user = strings.TrimSpace(user)
	if user == "" {
		return fmt.Errorf("请选择用户")
	}
	return account.SetAdmin(user, admin)
}

func (a *App) DisableAccountLockout() error {
	return account.DisableLockout()
}

func (a *App) EnableAccountLockout() error {
	return account.EnableLockout()
}

func (a *App) SetAccountLockoutPolicy(threshold, durationMin, windowMin int) error {
	if threshold < 0 || durationMin < 0 || windowMin < 0 {
		return fmt.Errorf("锁定策略参数无效")
	}
	return account.SetLockoutPolicy(uint32(threshold), uint32(durationMin), uint32(windowMin))
}

func (a *App) ChangeRdpPort(portNum uint32) error {
	if err := port.ValidTCP(portNum); err != nil {
		return err
	}
	if err := rdp.ValidatePort(portNum); err != nil {
		return err
	}
	return rdp.SetPort(portNum)
}

func (a *App) ToggleRdp() error {
	st, err := rdp.GetStatus()
	if err != nil {
		return err
	}
	return rdp.SetEnabled(!st.Enabled)
}

func (a *App) ClearRdpHistory() (string, error) {
	return rdp.ClearConnectionHistory()
}

func (a *App) ClearRdpHistoryByKind(kind string) (string, error) {
	return rdp.ClearConnectionHistoryByKind(kind)
}

func (a *App) GetRdpHistory() ([]rdp.HistoryEntry, error) {
	return rdp.ListConnectionHistory()
}

func (a *App) DeleteRdpHistoryEntry(kind, host, username, detail, sid string) error {
	return rdp.DeleteHistoryEntry(kind, host, username, detail, sid)
}

func (a *App) AllowFirewallPort(portNum uint32) error {
	if err := port.ValidTCP(portNum); err != nil {
		return err
	}
	return firewall.AllowTCP(portNum)
}

func (a *App) RemoveFirewallPort(portNum uint32) error {
	if err := port.ValidTCP(portNum); err != nil {
		return err
	}
	return firewall.RemoveAllowTCP(portNum)
}

func (a *App) ClearFirewallAllowRules() error {
	return firewall.ClearAllowRules()
}

func (a *App) SetFirewallEnabled(enabled bool) error {
	return firewall.SetAllProfiles(enabled)
}

func (a *App) DisablePing() error {
	return firewall.DisablePing()
}

func (a *App) EnablePing() error {
	return firewall.EnablePing()
}

func (a *App) ApplyTimeZone(id string) error {
	return wintime.SetTimeZone(id)
}

func (a *App) SaveNTPServer(server string) error {
	return wintime.SetNTPServer(server)
}

func (a *App) SyncNTP() error { return wintime.SyncNTP() }

func (a *App) TestNTPServer(server string) (string, error) {
	return wintime.TestNTPServer(server)
}

func (a *App) LockPC() error { return power.Lock() }

func (a *App) RestartPC(delay int) error {
	if delay < 0 || delay > power.MaxPowerDelaySeconds {
		return fmt.Errorf("延迟秒数无效（须为 0–%d）", power.MaxPowerDelaySeconds)
	}
	return power.Restart(delay)
}

func (a *App) ShutdownPC(delay int) error {
	if delay < 0 || delay > power.MaxPowerDelaySeconds {
		return fmt.Errorf("延迟秒数无效（须为 0–%d）", power.MaxPowerDelaySeconds)
	}
	return power.Shutdown(delay)
}

func (a *App) AbortPower() error { return power.Abort() }

func (a *App) DisableUpdate() error { return update.Disable() }

func (a *App) EnableUpdate() error { return update.Enable() }

func (a *App) DisableDefender() error { return defender.Disable() }

func (a *App) EnableDefender() error { return defender.Enable() }

func (a *App) CheckAppUpdate() (selfupdate.Info, error) {
	return selfupdate.Check(AppVersion)
}

func (a *App) DownloadAppUpdate() (selfupdate.Info, error) {
	return selfupdate.Download(AppVersion)
}

func (a *App) GetAppUpdateInfo() selfupdate.Info {
	return selfupdate.Cached()
}

func (a *App) ApplyAppUpdate() error {
	return selfupdate.Apply()
}
