package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wintoolbox/internal/account"
	"wintoolbox/internal/defender"
	"wintoolbox/internal/firewall"
	"wintoolbox/internal/harden"
	"wintoolbox/internal/rdp"
	"wintoolbox/internal/sysinfo"
	"wintoolbox/internal/update"
	"wintoolbox/internal/wintime"
)

const (
	overviewDetailTimeout  = 120 * time.Second
	statusCollectorTimeout = 15 * time.Second
)

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
	type lockoutRes struct {
		pol account.LockoutPolicy
		err error
	}
	type fwBundleRes struct {
		fw   firewall.ProfileStatus
		ping firewall.PingBlockStatus
		risk firewall.RiskBlockStatus
	}
	type guestRes struct {
		st account.GuestStatus
	}
	type autoRes struct {
		st account.AutoLogonStatus
	}
	type pwdRes struct {
		st account.PasswordPolicy
	}
	type hardenRes struct {
		st harden.Status
	}

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
		gotRisk     bool
		gotGuest    bool
		gotAuto     bool
		gotPwd      bool
		gotHarden   bool
	)

	var (
		overview  sysinfo.Overview
		accounts  []account.Info
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
		riskSt    firewall.RiskBlockStatus
		guestSt   account.GuestStatus
		autoSt    account.AutoLogonStatus
		pwdSt     account.PasswordPolicy
		hardenSt  harden.Status
	)

	fwSt = firewall.ProfileStatus{Domain: "超时", Private: "超时", Public: "超时"}
	pingSt = firewall.PingBlockStatus{}
	timeSt = wintime.Status{TimeZone: "-", NTPServer: "-", LocalTime: "-"}
	lockout = account.LockoutPolicy{Threshold: -1, Disabled: false, Unknown: true, Detail: "读取超时"}
	lockoutEr = fmt.Errorf("超时")
	riskSt = firewall.RiskBlockStatus{Detail: "读取超时"}
	guestSt = account.GuestStatus{Unknown: true, Detail: "读取超时"}
	autoSt = account.AutoLogonStatus{Unknown: true, Detail: "读取超时"}
	pwdSt = account.PasswordPolicy{MinLength: -1, ComplexityUnknown: true, Unknown: true, Detail: "读取超时"}
	hardenSt = harden.Status{
		Smb1Unknown: true, Smb1Detail: "读取超时",
		WinRMUnknown: true, WinRMDetail: "读取超时",
		AnonymousUnknown: true, AnonymousDetail: "读取超时",
	}

	chOverview := make(chan sysinfo.Overview, 1)
	chAcc := make(chan accRes, 1)
	chRdp := make(chan rdpRes, 1)
	chUpdate := make(chan update.Status, 1)
	chDef := make(chan defender.Status, 1)
	chFwBundle := make(chan fwBundleRes, 1)
	chTime := make(chan wintime.Status, 1)
	chLockout := make(chan lockoutRes, 1)
	chGuest := make(chan guestRes, 1)
	chAuto := make(chan autoRes, 1)
	chPwd := make(chan pwdRes, 1)
	chHarden := make(chan hardenRes, 1)

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
	// Serialize firewall probes to avoid PowerShell contention (faster + more reliable under 15s).
	go func() {
		chFwBundle <- fwBundleRes{
			fw:   firewall.GetProfiles(),
			ping: firewall.GetPingBlockStatus(),
			risk: firewall.GetRiskBlockStatus(),
		}
	}()
	go func() { chTime <- wintime.GetStatus() }()
	go func() {
		p, err := account.GetLockoutPolicy()
		chLockout <- lockoutRes{pol: p, err: err}
	}()
	go func() { chGuest <- guestRes{st: account.GetGuestStatus()} }()
	go func() { chAuto <- autoRes{st: account.GetAutoLogonStatus()} }()
	go func() { chPwd <- pwdRes{st: account.GetPasswordPolicy()} }()
	go func() { chHarden <- hardenRes{st: harden.GetStatus()} }()

	const moduleCount = 12
	received := 0
	for received < moduleCount {
		select {
		case <-ctx.Done():
			received = moduleCount
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
		case r := <-chFwBundle:
			gotFw, gotPing, gotRisk = true, true, true
			fwSt, pingSt, riskSt = r.fw, r.ping, r.risk
			received++
		case timeSt = <-chTime:
			gotTime = true
			received++
		case r := <-chLockout:
			gotLockout = true
			lockout, lockoutEr = r.pol, r.err
			received++
		case r := <-chGuest:
			gotGuest = true
			guestSt = r.st
			received++
		case r := <-chAuto:
			gotAuto = true
			autoSt = r.st
			received++
		case r := <-chPwd:
			gotPwd = true
			pwdSt = r.st
			received++
		case r := <-chHarden:
			gotHarden = true
			hardenSt = r.st
			received++
		}
	}

	pingTimedOut := !gotPing

	st := Status{
		Overview:         overviewFrom(overview),
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
		LockoutThreshold: -1,
		LockoutDuration:  -1,
		LockoutWindow:    -1,
		PasswordMinLength: -1,
		PasswordUnknown:   true,
		PasswordComplexityUnknown: true,
		PasswordPolicyDetail:      "读取超时",
	}

	if !gotAcc {
		st.Warnings = append(st.Warnings, "本地账户列表读取超时")
	} else if accErr != nil {
		st.Warnings = append(st.Warnings, "读取本地账户失败: "+accErr.Error())
	} else {
		st.Accounts = accountsFrom(accounts)
	}

	if gotRisk {
		st.RiskPortsBlocked = riskSt.AllBlocked
		st.RiskPortsPartial = riskSt.Partial
		st.RiskPortsUnknown = false
		st.RiskPortsDetail = riskSt.Detail
	} else {
		st.RiskPortsBlocked = false
		st.RiskPortsPartial = false
		st.RiskPortsUnknown = true
		st.RiskPortsDetail = "读取超时"
		st.Warnings = append(st.Warnings, "高危端口拦截状态读取超时")
	}

	if gotGuest {
		st.GuestExists = guestSt.Exists
		st.GuestEnabled = guestSt.Enabled
		st.GuestUnknown = guestSt.Unknown
		st.GuestDetail = guestSt.Detail
	} else {
		st.GuestUnknown = true
		st.GuestDetail = "读取超时"
	}

	if gotAuto {
		st.AutoLogonEnabled = autoSt.Enabled
		st.AutoLogonUnknown = autoSt.Unknown
		st.AutoLogonDetail = autoSt.Detail
	} else {
		st.AutoLogonUnknown = true
		st.AutoLogonDetail = "读取超时"
	}

	if gotPwd {
		st.PasswordMinLength = pwdSt.MinLength // may be -1 when unreadable
		st.PasswordComplexity = pwdSt.Complexity
		st.PasswordComplexityUnknown = pwdSt.ComplexityUnknown
		st.PasswordUnknown = pwdSt.Unknown
		st.PasswordPolicyDetail = pwdSt.Detail
	} else {
		st.PasswordMinLength = -1
		st.PasswordComplexity = false
		st.PasswordComplexityUnknown = true
		st.PasswordUnknown = true
		st.PasswordPolicyDetail = "读取超时"
		st.Warnings = append(st.Warnings, "密码策略读取超时")
	}

	if gotHarden {
		st.Smb1Disabled = hardenSt.Smb1Disabled
		st.Smb1Unknown = hardenSt.Smb1Unknown
		st.Smb1Detail = hardenSt.Smb1Detail
		st.WinRMHardened = hardenSt.WinRMHardened
		st.WinRMUnknown = hardenSt.WinRMUnknown
		st.WinRMDetail = hardenSt.WinRMDetail
		st.AnonymousOK = hardenSt.AnonymousOK
		st.AnonymousUnknown = hardenSt.AnonymousUnknown
		st.AnonymousDetail = hardenSt.AnonymousDetail
	} else {
		st.Smb1Unknown = true
		st.Smb1Detail = "读取超时"
		st.WinRMUnknown = true
		st.WinRMDetail = "读取超时"
		st.AnonymousUnknown = true
		st.AnonymousDetail = "读取超时"
		st.Warnings = append(st.Warnings, "安全加固状态读取超时")
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
		st.RdpNLA = rdpSt.NLA
		st.RdpNLAUnknown = rdpSt.NLAUnknown
	} else if gotRdp {
		st.Warnings = append(st.Warnings, "远程桌面状态读取失败: "+rdpErr.Error())
	} else {
		st.Warnings = append(st.Warnings, "远程桌面状态读取超时")
	}

	if gotLockout && lockoutEr == nil {
		st.LockoutDisabled = lockout.Disabled
		st.LockoutUnknown = lockout.Unknown
		st.LockoutDetail = lockout.Detail
		if !lockout.Unknown {
			if lockout.Threshold >= 0 {
				st.LockoutThreshold = lockout.Threshold
			}
			if n := parseLockoutMinutes(lockout.Duration); n >= 0 {
				st.LockoutDuration = n
			}
			if n := parseLockoutMinutes(lockout.Window); n >= 0 {
				st.LockoutWindow = n
			}
		}
		if lockout.Unknown {
			st.Warnings = append(st.Warnings, "账户锁定策略无法解析（系统语言可能不受支持）")
		}
	} else {
		st.LockoutDetail = "读取超时"
		st.LockoutUnknown = true
		if lockoutEr != nil {
			st.Warnings = append(st.Warnings, "账户锁定策略读取失败: "+lockoutEr.Error())
		} else {
			st.Warnings = append(st.Warnings, "账户锁定策略读取超时")
		}
	}

	if gotUpdate {
		st.UpdateUnknown = false
	} else {
		st.UpdateDisabled = false
		st.UpdateUnknown = true
		st.UpdateDetail = "读取超时"
		st.Warnings = append(st.Warnings, "系统更新状态读取超时")
	}
	if gotDefender {
		st.DefenderUnknown = defSt.Unknown
		if defSt.Unknown {
			st.Warnings = append(st.Warnings, "防病毒状态无法确定")
		}
	} else {
		st.DefenderDisabled = false
		st.DefenderUnknown = true
		st.DefenderDetail = "读取超时"
		st.Warnings = append(st.Warnings, "防病毒状态读取超时")
	}
	if gotTime {
		st.TimeUnknown = false
	} else {
		st.TimeUnknown = true
		st.TimeZone = ""
		st.NTPServer = ""
		st.TimeText = "时间状态读取超时"
		st.Warnings = append(st.Warnings, "时间/NTP 状态读取超时")
	}
	_ = gotOverview
	if !gotOverview {
		st.Warnings = append(st.Warnings, "系统概览读取超时")
	}
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
