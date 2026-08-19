package firewall

import (
	"fmt"
	"strconv"
	"strings"

	"wintoolbox/internal/win/port"
	"wintoolbox/internal/win/syscmd"
)

const rulePrefix = "WinToolbox-Allow-"
const (
	pingBlockRuleName   = "WinToolbox-Block-Ping"
	pingBlockRuleNameV4 = "WinToolbox-Block-Ping-IPv4"
	pingBlockRuleNameV6 = "WinToolbox-Block-Ping-IPv6"
)

// ProfileStatus describes whether Domain/Private/Public profiles are on.
type ProfileStatus struct {
	Domain  string
	Private string
	Public  string
	Raw     string
}

// RuleInfo is a WinToolbox allow rule summary.
type RuleInfo struct {
	Name    string `json:"name"`
	Port    string `json:"port"`
	Enabled bool   `json:"enabled"`
}

// PingBlockStatus describes whether WinToolbox ping block rules are active.
type PingBlockStatus struct {
	IPv4 bool
	IPv6 bool
}

// Mode returns enabled/partial/blocked for UI consumption.
func (s PingBlockStatus) Mode() string {
	switch {
	case s.IPv4 && s.IPv6:
		return "blocked"
	case s.IPv4 || s.IPv6:
		return "partial"
	default:
		return "enabled"
	}
}

// GetProfiles reads firewall profile enable state via netsh / PowerShell.
// Uses short timeouts so LoadStatus is not blocked for minutes.
func GetProfiles() ProfileStatus {
	// Prefer PowerShell for robustness against netsh localization differences.
	ps := profilesViaPowerShellQuick()
	if ps.Domain != "未知" && ps.Private != "未知" && ps.Public != "未知" {
		return ps
	}

	out, err := syscmd.RunQuick("netsh", "advfirewall", "show", "allprofiles")
	st := ProfileStatus{Domain: "未知", Private: "未知", Public: "未知", Raw: out}
	if err == nil {
		st.Domain = profileState(out, "Domain Profile", "域配置文件")
		st.Private = profileState(out, "Private Profile", "专用配置文件", "专用网络配置文件")
		st.Public = profileState(out, "Public Profile", "公用配置文件", "公用网络配置文件")
	}

	// Fill whatever PowerShell managed to parse.
	if ps.Domain != "未知" && st.Domain == "未知" {
		st.Domain = ps.Domain
	}
	if ps.Private != "未知" && st.Private == "未知" {
		st.Private = ps.Private
	}
	if ps.Public != "未知" && st.Public == "未知" {
		st.Public = ps.Public
	}
	if st.Raw == "" {
		st.Raw = ps.Raw
	}
	return st
}

func profilesViaPowerShellQuick() ProfileStatus {
	st := ProfileStatus{Domain: "未知", Private: "未知", Public: "未知"}
	// Reuse RunPS but the outer LoadStatus also caps wait; keep script tiny.
	out, err := syscmd.RunPSQuick(`Get-NetFirewallProfile | ForEach-Object { Write-Output ($_.Name + '=' + $_.Enabled) }`)
	if err != nil {
		return st
	}
	st.Raw = out
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		on := strings.EqualFold(val, "True") || val == "1" || strings.Contains(val, "True")
		off := strings.EqualFold(val, "False") || val == "0" || strings.Contains(val, "False")
		state := "未知"
		if on && !off {
			state = "开"
		} else if off {
			state = "关"
		}
		switch name {
		case "domain":
			st.Domain = state
		case "private":
			st.Private = state
		case "public":
			st.Public = state
		}
	}
	return st
}

func profileState(raw string, headers ...string) string {
	lines := strings.Split(raw, "\n")
	inSection := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if !inSection {
			for _, h := range headers {
				if strings.Contains(lower, strings.ToLower(h)) || strings.Contains(trim, h) {
					inSection = true
					break
				}
			}
			continue
		}
		// Skip separators; do not end the section on blank lines before State.
		if trim == "" || strings.HasPrefix(trim, "---") || strings.HasPrefix(trim, "===") {
			continue
		}
		if (strings.Contains(lower, "profile") || strings.Contains(trim, "配置文件")) &&
			!strings.Contains(lower, "state") && !strings.Contains(trim, "状态") {
			inSection = false
			continue
		}
		if strings.Contains(lower, "state") || strings.Contains(trim, "状态") {
			// zh-CN: 启用/禁用; some builds: 开/关; EN: on/off.
			fields := strings.Fields(trim)
			val := ""
			if len(fields) >= 2 {
				val = strings.ToLower(fields[len(fields)-1])
			}
			if strings.Contains(trim, "禁用") || val == "off" || val == "false" ||
				(strings.Contains(trim, "关") && !strings.Contains(trim, "开关")) {
				return "关"
			}
			if strings.Contains(trim, "启用") || val == "on" || val == "true" || strings.Contains(trim, "开") {
				return "开"
			}
		}
	}
	return "未知"
}

// Summary returns a short Chinese summary of profile states.
func (p ProfileStatus) Summary() string {
	return fmt.Sprintf("域=%s · 专用=%s · 公用=%s", p.Domain, p.Private, p.Public)
}

// AllEnabled reports whether all known profiles are on.
func (p ProfileStatus) AllEnabled() bool {
	return p.Domain == "开" && p.Private == "开" && p.Public == "开"
}

// AllDisabled reports whether all known profiles are off.
func (p ProfileStatus) AllDisabled() bool {
	return p.Domain == "关" && p.Private == "关" && p.Public == "关"
}

// SetAllProfiles enables or disables Domain/Private/Public firewall profiles.
func SetAllProfiles(enabled bool) error {
	flag := "False"
	state := "off"
	if enabled {
		flag = "True"
		state = "on"
	}

	ps := fmt.Sprintf(`
$ErrorActionPreference='Stop'
Set-NetFirewallProfile -Profile Domain,Private,Public -Enabled %s -ErrorAction Stop
Write-Output 'OK'
`, flag)
	out, err := syscmd.RunPS(ps)
	if err == nil && strings.Contains(out, "OK") {
		return nil
	}

	psMsg := strings.TrimSpace(out)
	if psMsg == "" && err != nil {
		psMsg = err.Error()
	}

	out2, err2 := syscmd.Run("netsh", "advfirewall", "set", "allprofiles", "state", state)
	if err2 != nil {
		msg2 := strings.TrimSpace(out2)
		if msg2 == "" {
			msg2 = err2.Error()
		}
		return fmt.Errorf("设置防火墙失败:\nPowerShell: %s\nnetsh: %s", psMsg, msg2)
	}
	return nil
}

// DisablePing blocks inbound ICMPv4/ICMPv6 echo requests on all firewall profiles.
func DisablePing() error {
	if err := disablePingRule(pingBlockRuleNameV4, "ICMPv4", "8"); err != nil {
		// Clean up IPv6 rule if the first step partially succeeded in a previous run.
		_ = removePingRule(pingBlockRuleNameV6)
		// Remove legacy combined-name rule if it exists but IPv4 dedicated rule failed to settle.
		_ = removePingRule(pingBlockRuleName)
		return err
	}
	if err := disablePingRule(pingBlockRuleNameV6, "ICMPv6", "128"); err != nil {
		_ = removePingRule(pingBlockRuleNameV4)
		_ = removePingRule(pingBlockRuleNameV6)
		_ = removePingRule(pingBlockRuleName)
		return err
	}
	// Remove the old IPv4-only legacy rule name after the dedicated rules are in place.
	_ = removePingRule(pingBlockRuleName)
	if !HasPingBlockRule() {
		return fmt.Errorf("禁 ping 规则已提交，但未校验到 IPv4/IPv6 均生效")
	}
	return nil
}

// EnablePing removes the WinToolbox ICMP echo block rules.
func EnablePing() error {
	for _, name := range []string{pingBlockRuleName, pingBlockRuleNameV4, pingBlockRuleNameV6} {
		_ = removePingRule(name)
	}
	if HasPingBlockRule() {
		return fmt.Errorf("恢复 ping 失败: IPv4/IPv6 禁 ping 规则仍存在")
	}
	return nil
}

func disablePingRule(name, protocol, icmpType string) error {
	ps := fmt.Sprintf(`
$ErrorActionPreference='Stop'
$name='%s'
$r = Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue
if ($r) {
  Set-NetFirewallRule -Name $name -Direction Inbound -Action Block -Enabled True -Profile Any -ErrorAction Stop | Out-Null
} else {
  New-NetFirewallRule -DisplayName $name -Name $name -Direction Inbound -Action Block -Protocol %s -IcmpType %s -Profile Any -Enabled True -Description 'WinToolbox block ping rule' | Out-Null
}
$check = Get-NetFirewallRule -Name $name -ErrorAction Stop
if ($check.Enabled -ne 'True' -and $check.Enabled -ne $true) { throw 'rule not enabled' }
if ($check.Action -ne 'Block' -and $check.Action -ne 4) { throw 'rule not blocking' }
Write-Output 'OK'
`, name, protocol, icmpType)
	out, err := syscmd.RunPS(ps)
	if err == nil && strings.Contains(out, "OK") {
		return nil
	}

	psMsg := strings.TrimSpace(out)
	if psMsg == "" && err != nil {
		psMsg = err.Error()
	}

	netshProto := strings.ToLower(protocol) + ":" + icmpType + ",any"
	out2, err2 := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+name,
		"dir=in",
		"action=block",
		"enable=yes",
		"profile=domain,private,public",
		"protocol="+netshProto,
	)
	if err2 != nil {
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "set", "rule",
			"name="+name,
			"new",
			"enable=yes",
			"action=block",
			"dir=in",
			"profile=domain,private,public",
		)
		if hasPingBlockRuleName(name) {
			return nil
		}
		msg2 := strings.TrimSpace(out2)
		if msg2 == "" {
			msg2 = err2.Error()
		}
		return fmt.Errorf("禁 ping 失败（%s）:\nPowerShell: %s\nnetsh: %s", name, psMsg, msg2)
	}
	if !hasPingBlockRuleName(name) {
		return fmt.Errorf("禁 ping 规则已提交，但未校验到生效规则（%s）", name)
	}
	return nil
}

func removePingRule(name string) error {
	_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	_, _ = syscmd.RunPS(fmt.Sprintf(
		`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
		name,
	))
	if hasPingBlockRuleName(name) {
		return fmt.Errorf("删除禁 ping 规则失败: %s 仍存在", name)
	}
	return nil
}

// HasPingBlockRule reports whether the WinToolbox ping block rules for IPv4 and IPv6 are both enabled.
func HasPingBlockRule() bool {
	st := GetPingBlockStatus()
	return st.IPv4 && st.IPv6
}

// GetPingBlockStatus reports the active state of the WinToolbox ping block rules.
func GetPingBlockStatus() PingBlockStatus {
	return PingBlockStatus{
		IPv4: hasPingBlockRuleName(pingBlockRuleNameV4),
		IPv6: hasPingBlockRuleName(pingBlockRuleNameV6),
	}
}

func hasPingBlockRuleName(name string) bool {
	out, err := syscmd.Run("netsh", "advfirewall", "firewall", "show", "rule", "name="+name, "verbose")
	if err == nil && strings.Contains(out, name) &&
		(strings.Contains(strings.ToLower(out), "block") || strings.Contains(out, "阻止")) &&
		netshRuleEnabled(out) {
		return true
	}

	ps := fmt.Sprintf(`
$r=Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue
if(-not $r){Write-Output 'NO'; exit 0}
Write-Output ("ENABLED=" + $r.Enabled + ";ACTION=" + $r.Action)
`, name)
	out, err = syscmd.RunPS(ps)
	if err != nil {
		return false
	}
	return (strings.Contains(out, "ENABLED=True") || strings.Contains(out, "ENABLED=true")) &&
		(strings.Contains(out, "ACTION=Block") || strings.Contains(out, "ACTION=4") || strings.Contains(strings.ToLower(out), "block"))
}

func ruleName(port uint32) string {
	return rulePrefix + strconv.FormatUint(uint64(port), 10)
}

// AllowTCP opens an inbound allow rule for the given TCP port (all profiles).
// Existing rules are updated in place; the old rule is only removed after a
// successful create when using the netsh fallback path.
func AllowTCP(p uint32) error {
	if err := port.ValidTCP(p); err != nil {
		return err
	}
	name := ruleName(p)
	portStr := strconv.FormatUint(uint64(p), 10)

	var errs []string

	// Prefer PowerShell upsert: update existing rule or create new — never delete first.
	ps := fmt.Sprintf(`
$ErrorActionPreference='Stop'
$name='%s'
$port=%s
$r = Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue
if ($r) {
  Set-NetFirewallRule -Name $name -Direction Inbound -Action Allow -Enabled True -Profile Any -ErrorAction Stop | Out-Null
  $r | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort $port -ErrorAction Stop | Out-Null
} else {
  New-NetFirewallRule -DisplayName $name -Name $name -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any -Enabled True -Description 'WinToolbox allow rule' | Out-Null
}
$check = Get-NetFirewallRule -Name $name -ErrorAction Stop
$pf = $check | Get-NetFirewallPortFilter
if ($check.Enabled -ne 'True' -and $check.Enabled -ne $true) { throw 'rule not enabled' }
$local = [string]$pf.LocalPort
if ($local -ne $port) { throw ("port mismatch: " + $local) }
Write-Output 'OK'
`, name, portStr)
	if out, err := syscmd.RunPS(ps); err != nil || !strings.Contains(out, "OK") {
		msg := strings.TrimSpace(out)
		if msg == "" && err != nil {
			msg = err.Error()
		}
		errs = append(errs, "PowerShell: "+msg)

		// Fallback: netsh add. Only delete old rule AFTER successful add of a temp name,
		// then rename by delete+add of final name — use temp rule to avoid gap.
		tmpName := name + "-tmp"
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+tmpName)
		out2, err2 := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
			"name="+tmpName,
			"dir=in",
			"action=allow",
			"protocol=TCP",
			"localport="+portStr,
			"profile=domain,private,public",
			"enable=yes",
		)
		if err2 != nil {
			msg2 := strings.TrimSpace(out2)
			if msg2 == "" {
				msg2 = err2.Error()
			}
			errs = append(errs, "netsh: "+msg2)
			return fmt.Errorf("创建防火墙规则失败:\n%s", strings.Join(errs, "\n"))
		}
		// Temp rule is live — safe to replace the old named rule.
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
		_, _ = syscmd.RunPS(fmt.Sprintf(
			`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
			name,
		))
		out3, err3 := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
			"name="+name,
			"dir=in",
			"action=allow",
			"protocol=TCP",
			"localport="+portStr,
			"profile=domain,private,public",
			"enable=yes",
		)
		if err3 != nil {
			// Prefer keeping the temp rule only briefly; surface it so UI can list/remove *-tmp.
			_ = out3
			return fmt.Errorf("创建防火墙规则失败（临时规则 %s 仍可能有效，可在规则列表中删除）:\n%s", tmpName, strings.Join(errs, "\n"))
		}
		for attempt := 0; attempt < 2; attempt++ {
			_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+tmpName)
			_, _ = syscmd.RunPS(fmt.Sprintf(
				`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
				tmpName,
			))
			if !hasNamedAllowRule(tmpName) {
				break
			}
		}
		if hasNamedAllowRule(tmpName) {
			return fmt.Errorf("主规则 %s 已创建，但临时规则 %s 未能删除，请在规则列表中手动删除", name, tmpName)
		}
	}

	if !HasAllowTCP(p) {
		return fmt.Errorf("规则已提交，但未校验到生效的入站放行规则（%s，TCP %s）。请确认已以管理员运行，且未被组策略禁止本地防火墙规则", name, portStr)
	}
	return nil
}

// HasAllowTCP reports whether the WinToolbox allow rule for port exists and looks enabled.
func HasAllowTCP(port uint32) bool {
	name := ruleName(port)
	portStr := strconv.FormatUint(uint64(port), 10)

	out, err := syscmd.Run("netsh", "advfirewall", "firewall", "show", "rule", "name="+name, "verbose")
	if err == nil && strings.Contains(out, name) &&
		(strings.Contains(strings.ToLower(out), "allow") || strings.Contains(out, "允许")) &&
		netshRulePortMatches(out, portStr) && netshRuleEnabled(out) {
		return true
	}

	ps := fmt.Sprintf(`
$r=Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue
if(-not $r){Write-Output 'NO'; exit 0}
$p=$r|Get-NetFirewallPortFilter
Write-Output ("ENABLED=" + $r.Enabled + ";PORT=" + $p.LocalPort + ";ACTION=" + $r.Action)
`, name)
	out, err = syscmd.RunPS(ps)
	if err != nil {
		return false
	}
	return strings.Contains(out, "PORT="+portStr) &&
		(strings.Contains(out, "ENABLED=True") || strings.Contains(out, "ENABLED=true")) &&
		(strings.Contains(out, "ACTION=Allow") || strings.Contains(out, "ACTION=2") || strings.Contains(strings.ToLower(out), "allow"))
}

// ListAllowRules lists WinToolbox-Allow-* inbound rules.
func ListAllowRules() []RuleInfo {
	out, err := syscmd.RunPS(`
Get-NetFirewallRule -ErrorAction SilentlyContinue |
  Where-Object { $_.DisplayName -like 'WinToolbox-Allow-*' -or $_.Name -like 'WinToolbox-Allow-*' } |
  ForEach-Object {
    $p = $_ | Get-NetFirewallPortFilter
    $en = '0'
    if ($_.Enabled -eq 'True' -or $_.Enabled -eq $true) { $en = '1' }
    Write-Output ($_.Name + [char]9 + [string]$p.LocalPort + [char]9 + $en)
  }
`)
	if err != nil || strings.TrimSpace(out) == "" {
		return listAllowRulesNetsh()
	}
	var rules []RuleInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		portStr, ok := parseSinglePortStr(parts[1])
		if !ok {
			// Only list exact single ports so "按端口删除" works reliably.
			continue
		}
		rules = append(rules, RuleInfo{
			Name:    parts[0],
			Port:    portStr,
			Enabled: len(parts) < 3 || parts[2] == "1",
		})
	}
	return rules
}

// parseSinglePortStr parses a single TCP port number in [1..65535].
// If the string contains ranges (e.g. "80-90") or lists (e.g. "80,81"), it returns false.
func parseSinglePortStr(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n < 1 || n > 65535 {
		return "", false
	}
	return strconv.FormatUint(n, 10), true
}

func listAllowRulesNetsh() []RuleInfo {
	out, err := syscmd.Run("netsh", "advfirewall", "firewall", "show", "rule", "name=all", "dir=in")
	if err != nil {
		return nil
	}
	var rules []RuleInfo
	var cur NamePort
	flush := func() {
		if strings.HasPrefix(cur.Name, rulePrefix) && cur.Port != "" {
			portStr, ok := parseSinglePortStr(cur.Port)
			if !ok {
				cur = NamePort{}
				return
			}
			rules = append(rules, RuleInfo{Name: cur.Name, Port: portStr, Enabled: cur.Enabled})
		}
		cur = NamePort{}
	}
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "规则名称:") || strings.HasPrefix(strings.ToLower(trim), "rule name:") {
			flush()
			parts := strings.SplitN(trim, ":", 2)
			if len(parts) == 2 {
				cur.Name = strings.TrimSpace(parts[1])
			}
			continue
		}
		if strings.Contains(trim, "本地端口") || strings.HasPrefix(strings.ToLower(trim), "local port") {
			parts := strings.SplitN(trim, ":", 2)
			if len(parts) == 2 {
				cur.Port = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(trim, "已启用") || strings.HasPrefix(strings.ToLower(trim), "enabled") {
			cur.Enabled = !(strings.Contains(trim, "否") || strings.Contains(strings.ToLower(trim), "no"))
		}
	}
	flush()
	return rules
}

type NamePort struct {
	Name    string
	Port    string
	Enabled bool
}

// RemoveAllowTCP deletes the WinToolbox allow rule for a port if present,
// including the legacy/fallback temporary name WinToolbox-Allow-<port>-tmp.
func RemoveAllowTCP(p uint32) error {
	if err := port.ValidTCP(p); err != nil {
		return err
	}
	name := ruleName(p)
	tmpName := name + "-tmp"
	for _, n := range []string{name, tmpName} {
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+n)
		_, _ = syscmd.RunPS(fmt.Sprintf(
			`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
			n,
		))
	}
	if HasAllowTCP(p) || hasNamedAllowRule(tmpName) {
		return fmt.Errorf("删除防火墙规则失败: 规则仍存在（含临时规则时请重试）")
	}
	return nil
}

// ClearAllowRules removes all inbound WinToolbox-Allow-* rules (including legacy tmp names).
func ClearAllowRules() error {
	out, err := syscmd.RunPS(`
$ErrorActionPreference='Stop'
$rules = Get-NetFirewallRule -ErrorAction SilentlyContinue |
  Where-Object { $_.DisplayName -like 'WinToolbox-Allow-*' -or $_.Name -like 'WinToolbox-Allow-*' }
$before = @($rules).Count
if ($before -eq 0) { Write-Output 'NO'; exit 0 }
$rules | Remove-NetFirewallRule -ErrorAction SilentlyContinue
$afterRules = Get-NetFirewallRule -ErrorAction SilentlyContinue |
  Where-Object { $_.DisplayName -like 'WinToolbox-Allow-*' -or $_.Name -like 'WinToolbox-Allow-*' }
$after = @($afterRules).Count
Write-Output ('OK=' + $before + ':' + $after)
`)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("删除防火墙放行规则失败: %s", msg)
	}
	out = strings.TrimSpace(out)
	if out == "NO" {
		return nil
	}
	if strings.HasPrefix(out, "OK=") {
		parts := strings.Split(strings.TrimPrefix(out, "OK="), ":")
		if len(parts) == 2 {
			if strings.TrimSpace(parts[1]) == "0" {
				return nil
			}
			return fmt.Errorf("删除防火墙放行规则失败: 删除后仍存在 %s 条", strings.TrimSpace(parts[1]))
		}
	}
	return fmt.Errorf("删除防火墙放行规则失败: %s", out)
}

func hasNamedAllowRule(name string) bool {
	out, err := syscmd.Run("netsh", "advfirewall", "firewall", "show", "rule", "name="+name)
	if err != nil {
		return false
	}
	return strings.Contains(out, name)
}

func netshRulePortMatches(out, portStr string) bool {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(lower, "localport") && !strings.Contains(line, "本地端口") {
			continue
		}
		if portInNetshValue(line, portStr) {
			return true
		}
	}
	return strings.Contains(out, portStr)
}

func portInNetshValue(line, portStr string) bool {
	want, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil {
		return strings.Contains(line, portStr)
	}
	if idx := strings.Index(line, ":"); idx >= 0 {
		val := strings.TrimSpace(line[idx+1:])
		for _, part := range strings.Split(val, ",") {
			part = strings.TrimSpace(part)
			if part == portStr {
				return true
			}
			if strings.Contains(part, "-") {
				bounds := strings.SplitN(part, "-", 2)
				if len(bounds) != 2 {
					continue
				}
				lo, err1 := strconv.ParseUint(strings.TrimSpace(bounds[0]), 10, 32)
				hi, err2 := strconv.ParseUint(strings.TrimSpace(bounds[1]), 10, 32)
				if err1 == nil && err2 == nil && want >= lo && want <= hi {
					return true
				}
			}
		}
	}
	return strings.Contains(line, portStr)
}

func netshRuleEnabled(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(line, "已启用") {
			return strings.Contains(line, "是") || strings.Contains(strings.ToLower(line), "yes")
		}
		if strings.Contains(lower, "enabled") {
			return strings.Contains(lower, "yes") || strings.Contains(lower, "true")
		}
	}
	return true
}
