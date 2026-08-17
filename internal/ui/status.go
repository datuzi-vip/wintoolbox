package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"wintoolbox/internal/account"
	"wintoolbox/internal/defender"
	"wintoolbox/internal/firewall"
	"wintoolbox/internal/rdp"
	"wintoolbox/internal/sysinfo"
	"wintoolbox/internal/update"
	"wintoolbox/internal/wintime"
)

const (
	overviewDetailTimeout = 120 * time.Second
	statusCollectorTimeout = 15 * time.Second
)

type statusCollect struct {
	overview  sysinfo.Overview
	accounts  []account.Info
	accErr    error
	rdp       rdp.Status
	rdpErr    error
	update    update.Status
	defender  defender.Status
	fw        firewall.ProfileStatus
	time      wintime.Status
	lockout   account.LockoutPolicy
	lockoutEr error
}

// LoadStatus gathers a fast snapshot for first paint.
func LoadStatus(invalidateOverview bool) (Status, error) {
	if invalidateOverview {
		sysinfo.InvalidateOverviewCache()
	}

	c := &statusCollect{}
	var wg sync.WaitGroup
	wg.Add(8)
	go func() { defer wg.Done(); c.overview = sysinfo.GetOverviewFast() }()
	go func() {
		defer wg.Done()
		c.accounts, c.accErr = account.ListAccounts()
	}()
	go func() {
		defer wg.Done()
		c.rdp, c.rdpErr = rdp.GetStatus()
	}()
	go func() { defer wg.Done(); c.update = update.GetStatus() }()
	go func() { defer wg.Done(); c.defender = defender.GetStatus() }()
	go func() {
		defer wg.Done()
		done := make(chan firewall.ProfileStatus, 1)
		go func() { done <- firewall.GetProfiles() }()
		select {
		case c.fw = <-done:
		case <-time.After(statusCollectorTimeout):
			c.fw = firewall.ProfileStatus{Domain: "超时", Private: "超时", Public: "超时"}
		}
	}()
	go func() { defer wg.Done(); c.time = wintime.GetStatus() }()
	go func() {
		defer wg.Done()
		c.lockout, c.lockoutEr = account.GetLockoutPolicy()
	}()
	wg.Wait()

	if c.accErr != nil {
		return Status{}, fmt.Errorf("读取本地账户失败: %w", c.accErr)
	}

	st := Status{
		Overview:         overviewFrom(c.overview),
		Accounts:         accountsFrom(c.accounts),
		RdpAvailable:     c.rdpErr == nil,
		TimeZones:        wintime.EnsureZoneOption(wintime.CommonZones(), c.time.TimeZone),
		FirewallSummary:  c.fw.Summary(),
		FirewallDomain:   c.fw.Domain,
		FirewallPrivate:  c.fw.Private,
		FirewallPublic:   c.fw.Public,
		FirewallAllOn:    c.fw.AllEnabled(),
		FirewallAllOff:   c.fw.AllDisabled(),
		UpdateDisabled:   c.update.Disabled,
		UpdateDetail:     c.update.Detail,
		DefenderDisabled: c.defender.Disabled,
		DefenderDetail:   c.defender.Detail,
		TimeZone:         c.time.TimeZone,
		NTPServer:        c.time.NTPServer,
		TimeText:         fmt.Sprintf("%s  ·  %s  ·  NTP %s", c.time.LocalTime, c.time.TimeZone, c.time.NTPServer),
	}
	if c.fw.Domain == "超时" || c.fw.Private == "超时" || c.fw.Public == "超时" {
		st.Warnings = append(st.Warnings, "防火墙配置文件状态读取超时")
	}
	if c.rdpErr == nil {
		st.RdpEnabled, st.RdpPort = c.rdp.Enabled, c.rdp.Port
	} else {
		st.Warnings = append(st.Warnings, "远程桌面状态读取失败: "+c.rdpErr.Error())
	}
	if c.lockoutEr == nil {
		st.LockoutDisabled = c.lockout.Disabled
		st.LockoutUnknown = c.lockout.Unknown
		st.LockoutDetail = c.lockout.Detail
		if c.lockout.Unknown {
			st.Warnings = append(st.Warnings, "账户锁定策略无法解析（系统语言可能不受支持）")
		}
	} else {
		st.LockoutDetail = "读取失败"
		st.LockoutUnknown = true
		st.Warnings = append(st.Warnings, "账户锁定策略读取失败: "+c.lockoutEr.Error())
	}
	return st, nil
}

// LoadOverviewDetail fills slow overview fields (with timeout).
func LoadOverviewDetail() (OverviewDetail, error) {
	type result struct {
		d   OverviewDetail
		err error
	}
	ch := make(chan result, 1)
	go func() {
		d := sysinfo.GetOverviewDetail()
		physTexts := formatPhysicalDisks(d.PhysicalDisks)
		ch <- result{
			d: OverviewDetail{
				MemoryModules:    dash(d.MemoryModules),
				PhysicalDisks:    physTexts,
				GPUs:             d.GPUs,
				Activated:        d.Activated,
				ActivationStatus: dash(d.ActivationStatus),
			},
		}
	}()

	select {
	case r := <-ch:
		return r.d, r.err
	case <-time.After(overviewDetailTimeout):
		return OverviewDetail{
			MemoryModules:    "获取超时",
			ActivationStatus: "获取超时",
		}, fmt.Errorf("硬件详情采集超时（%v）", overviewDetailTimeout)
	}
}

// LoadTimeZones returns full timezone list for the time page.
func LoadTimeZones() ([]wintime.ZoneOption, error) {
	zones, err := wintime.ListTimeZones()
	if err != nil || len(zones) == 0 {
		zones = wintime.CommonZones()
	}
	return wintime.EnsureZoneOption(zones, wintime.CurrentTimeZone()), nil
}

// LoadFirewallRules returns WinToolbox allow rules.
func LoadFirewallRules() ([]firewall.RuleInfo, error) {
	rules := firewall.ListAllowRules()
	return rules, nil
}

func accountsFrom(accs []account.Info) []AccountView {
	me := account.CurrentUsername()
	out := make([]AccountView, 0, len(accs))
	for _, a := range accs {
		out = append(out, AccountView{
			Name: a.Name, Enabled: a.Enabled, EnabledUnknown: a.EnabledUnknown,
			Admin: a.Admin, AdminUnknown: a.AdminUnknown,
			Current: strings.EqualFold(a.Name, me),
		})
	}
	return out
}

func overviewFrom(ov sysinfo.Overview) OverviewView {
	diskTexts := make([]string, 0, len(ov.Disks))
	for _, d := range ov.Disks {
		diskTexts = append(diskTexts, fmt.Sprintf("%s %.0f/%.0f GB", d.Root, d.FreeGB, d.TotalGB))
	}
	return OverviewView{
		Hostname: ov.Hostname, OSName: ov.OSName, OSBuild: ov.OSBuild, Arch: ov.Arch,
		Manufacturer: ov.Manufacturer, Model: ov.Model, Board: ov.Board, BIOS: ov.BIOS,
		CPU: ov.CPU, CPUCores: ov.CPUCores, MemoryTotalGB: ov.MemoryTotalGB, MemoryAvailGB: ov.MemoryAvailGB,
		MemoryModules: ov.MemoryModules, Resolution: ov.Resolution, IPs: ov.IPs,
		Disks: diskTexts, PhysicalDisks: formatPhysicalDisks(ov.PhysicalDisks), GPUs: ov.GPUs,
		Activated: ov.Activated, ActivationStatus: ov.ActivationStatus,
	}
}

func formatPhysicalDisks(disks []sysinfo.PhysicalDisk) []string {
	physTexts := make([]string, 0, len(disks))
	for _, d := range disks {
		if d.Media != "" {
			physTexts = append(physTexts, fmt.Sprintf("%s（%s，%.0f GB）", d.Model, d.Media, d.SizeGB))
		} else {
			physTexts = append(physTexts, fmt.Sprintf("%s（%.0f GB）", d.Model, d.SizeGB))
		}
	}
	return physTexts
}
