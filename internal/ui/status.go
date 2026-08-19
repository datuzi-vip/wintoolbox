package ui

import (
	"context"
	"fmt"
	"strings"
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
	overviewDetailTimeout  = 120 * time.Second
	statusCollectorTimeout = 15 * time.Second
)

type statusCollect struct {
	overview     sysinfo.Overview
	accounts     []account.Info
	accErr       error
	rdp          rdp.Status
	rdpErr       error
	update       update.Status
	defender     defender.Status
	fw           firewall.ProfileStatus
	time         wintime.Status
	ping         firewall.PingBlockStatus
	pingTimedOut bool
	lockout      account.LockoutPolicy
	lockoutEr    error
}

// LoadStatus gathers a fast snapshot for first paint.
func LoadStatus(invalidateOverview bool) (Status, error) {
	if invalidateOverview {
		sysinfo.InvalidateOverviewCache()
	}

	type accRes struct {
		accounts []account.Info
		err      error
	}
	type rdpRes struct {
		st  rdp.Status
		err error
	}
	type fwRes struct {
		st firewall.ProfileStatus
	}
	type pingRes struct {
		st firewall.PingBlockStatus
	}
	type lockoutRes struct {
		pol account.LockoutPolicy
		err error
	}

	// Avoid blocking UI refresh: only wait up to statusCollectorTimeout for all modules.
	// Each goroutine writes to its buffered channel once, preventing data races.
	ctx, cancel := context.WithTimeout(context.Background(), statusCollectorTimeout)
	defer cancel()

	var (
		gotOverview bool
		gotAcc      bool
		gotRdp      bool
		gotUpdate   bool
		gotDefender bool
		gotFw       bool
		gotTime     bool
		gotPing     bool
		gotLockout  bool
	)

	var (
		overview sysinfo.Overview
		accounts []account.Info
		accErr    error
		rdpSt     rdp.Status
		rdpErr    error
		upd       update.Status
		defSt     defender.Status
		fwSt      firewall.ProfileStatus
		timeSt    wintime.Status
		pingSt    firewall.PingBlockStatus
		lockout   account.LockoutPolicy
		lockoutEr error
	)

	// Defaults for timeout paths (keep UI safe & predictable).
	fwSt = firewall.ProfileStatus{Domain: "超时", Private: "超时", Public: "超时"}
	pingSt = firewall.PingBlockStatus{} // false/false => UI "partial/enabled" depends on Mode(); we'll override state below
	timeSt = wintime.Status{TimeZone: "-", NTPServer: "-", LocalTime: "-"}
	lockout = account.LockoutPolicy{Threshold: -1, Disabled: false, Unknown: true, Detail: "读取超时"}
	lockoutEr = fmt.Errorf("超时")

	chOverview := make(chan sysinfo.Overview, 1)
	chAcc := make(chan accRes, 1)
	chRdp := make(chan rdpRes, 1)
	chUpdate := make(chan update.Status, 1)
	chDef := make(chan defender.Status, 1)
	chFw := make(chan fwRes, 1)
	chTime := make(chan wintime.Status, 1)
	chPing := make(chan pingRes, 1)
	chLockout := make(chan lockoutRes, 1)

	go func() { chOverview <- sysinfo.GetOverviewFast() }()
	go func() {
		a, err := account.ListAccounts()
		chAcc <- accRes{accounts: a, err: err}
	}()
	go func() {
		s, err := rdp.GetStatus()
		chRdp <- rdpRes{st: s, err: err}
	}()
	go func() { chUpdate <- update.GetStatus() }()
	go func() { chDef <- defender.GetStatus() }()
	go func() { chFw <- fwRes{st: firewall.GetProfiles()} }()
	go func() { chTime <- wintime.GetStatus() }()
	go func() { chPing <- pingRes{st: firewall.GetPingBlockStatus()} }()
	go func() {
		p, err := account.GetLockoutPolicy()
		chLockout <- lockoutRes{pol: p, err: err}
	}()

	const moduleCount = 9
	received := 0
	for received < moduleCount {
		select {
		case <-ctx.Done():
			received = moduleCount // stop waiting; remaining modules keep timeout defaults
		case overview = <-chOverview:
			gotOverview = true
			received++
		case r := <-chAcc:
			gotAcc = true
			accounts, accErr = r.accounts, r.err
			received++
		case r := <-chRdp:
			gotRdp = true
			rdpSt, rdpErr = r.st, r.err
			received++
		case upd = <-chUpdate:
			gotUpdate = true
			received++
		case defSt = <-chDef:
			gotDefender = true
			received++
		case r := <-chFw:
			gotFw = true
			fwSt = r.st
			received++
		case timeSt = <-chTime:
			gotTime = true
			received++
		case r := <-chPing:
			gotPing = true
			pingSt = r.st
			received++
		case r := <-chLockout:
			gotLockout = true
			lockout, lockoutEr = r.pol, r.err
			received++
		}
	}

	// Accounts are required for UI correctness.
	if accErr != nil || !gotAcc {
		if accErr == nil {
			accErr = fmt.Errorf("读取超时")
		}
		return Status{}, fmt.Errorf("读取本地账户失败: %w", accErr)
	}

	// Firewall profile/ping may be partial due to timeout; reflect it in UI semantics.
	pingTimedOut := !gotPing

	st := Status{
		Overview:         overviewFrom(overview),
		Accounts:         accountsFrom(accounts),
		RdpAvailable:     rdpErr == nil && gotRdp,
		TimeZones:        wintime.EnsureZoneOption(wintime.CommonZones(), timeSt.TimeZone),
		FirewallSummary:  fwSt.Summary(),
		FirewallDomain:   fwSt.Domain,
		FirewallPrivate:  fwSt.Private,
		FirewallPublic:   fwSt.Public,
		FirewallAllOn:    fwSt.AllEnabled(),
		FirewallAllOff:   fwSt.AllDisabled(),
		PingBlocked:      pingSt.IPv4 && pingSt.IPv6,
		PingIPv4Blocked:  pingSt.IPv4,
		PingIPv6Blocked:  pingSt.IPv6,
		PingState:        pingSt.Mode(),
		UpdateDisabled:   upd.Disabled,
		UpdateDetail:     upd.Detail,
		DefenderDisabled: defSt.Disabled,
		DefenderDetail:   defSt.Detail,
		TimeZone:         timeSt.TimeZone,
		NTPServer:        timeSt.NTPServer,
		TimeText:         fmt.Sprintf("%s  ·  %s  ·  NTP %s", timeSt.LocalTime, timeSt.TimeZone, timeSt.NTPServer),
		LockoutThreshold: account.DefaultEnableThreshold,
		LockoutDuration:  account.DefaultEnableDuration,
		LockoutWindow:    account.DefaultEnableWindow,
	}

	if !gotFw || fwSt.Domain == "超时" || fwSt.Private == "超时" || fwSt.Public == "超时" {
		st.Warnings = append(st.Warnings, "防火墙配置文件状态读取超时")
	}
	if pingTimedOut {
		st.PingState = "unknown"
		st.Warnings = append(st.Warnings, "禁 ping 状态读取超时")
	}

	if gotRdp && rdpErr == nil {
		st.RdpEnabled, st.RdpPort = rdpSt.Enabled, rdpSt.Port
	} else if gotRdp {
		st.Warnings = append(st.Warnings, "远程桌面状态读取失败: "+rdpErr.Error())
	} else {
		st.Warnings = append(st.Warnings, "远程桌面状态读取超时")
	}

	if gotLockout && lockoutEr == nil {
		st.LockoutDisabled = lockout.Disabled
		st.LockoutUnknown = lockout.Unknown
		st.LockoutDetail = lockout.Detail
		if lockout.Threshold >= 0 {
			st.LockoutThreshold = lockout.Threshold
		}
		if n := parseLockoutMinutes(lockout.Duration); n >= 0 {
			st.LockoutDuration = n
		}
		if n := parseLockoutMinutes(lockout.Window); n >= 0 {
			st.LockoutWindow = n
		}
		if lockout.Unknown {
			st.Warnings = append(st.Warnings, "账户锁定策略无法解析（系统语言可能不受支持）")
		}
	} else {
		st.LockoutDetail = "读取超时"
		st.LockoutUnknown = true
		if lockoutEr != nil {
			st.Warnings = append(st.Warnings, "账户锁定策略读取失败: "+lockoutEr.Error())
		}
	}

	_ = gotOverview
	_ = gotUpdate
	_ = gotDefender
	_ = gotTime
	return st, nil
}

func parseLockoutMinutes(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return -1
	}
	if strings.EqualFold(s, "never") {
		return -1
	}
	n := 0
	found := false
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
			found = true
			continue
		}
		if found {
			break
		}
	}
	if !found {
		return -1
	}
	return n
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
