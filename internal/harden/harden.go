package harden

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/syscmd"
)

const (
	lsaKeyPath    = `SYSTEM\CurrentControlSet\Control\Lsa`
	smb1KeyPath   = `SYSTEM\CurrentControlSet\Services\LanmanServer\Parameters`
	winRMService  = "WinRM"
	winRMBlockV4  = "WinToolbox-Block-WinRM-5985"
	winRMBlockV6  = "WinToolbox-Block-WinRM-5986"
)

// Status aggregates SMBv1 / WinRM / anonymous enumeration hardening state.
type Status struct {
	Smb1Disabled     bool
	Smb1Unknown      bool
	Smb1Detail       string
	WinRMHardened    bool
	WinRMUnknown     bool
	WinRMDetail      string
	AnonymousOK      bool
	AnonymousUnknown bool
	AnonymousDetail  string
}

// GetStatus reads current hardening status (best-effort, parallel; no shared writes).
func GetStatus() Status {
	type smbRes struct {
		disabled, unknown bool
		detail            string
	}
	type winRes struct {
		hardened, unknown bool
		detail            string
	}
	type anonRes struct {
		ok, unknown bool
		detail      string
	}

	var wg sync.WaitGroup
	var smb smbRes
	var win winRes
	var anon anonRes
	wg.Add(3)
	go func() {
		defer wg.Done()
		smb.disabled, smb.unknown, smb.detail = smb1State()
	}()
	go func() {
		defer wg.Done()
		win.hardened, win.unknown, win.detail = winrmState()
	}()
	go func() {
		defer wg.Done()
		anon.ok, anon.unknown, anon.detail = anonymousState()
	}()
	wg.Wait()

	return Status{
		Smb1Disabled:     smb.disabled,
		Smb1Unknown:      smb.unknown,
		Smb1Detail:       smb.detail,
		WinRMHardened:    win.hardened,
		WinRMUnknown:     win.unknown,
		WinRMDetail:      win.detail,
		AnonymousOK:      anon.ok,
		AnonymousUnknown: anon.unknown,
		AnonymousDetail:  anon.detail,
	}
}

func smb1State() (disabled, unknown bool, detail string) {
	// Prefer SMB server configuration.
	out, err := syscmd.RunPSQuick(`
$ErrorActionPreference='SilentlyContinue'
try {
  $c = Get-SmbServerConfiguration -ErrorAction Stop
  Write-Output ('SMB1=' + $c.EnableSMB1Protocol)
} catch {
  Write-Output 'ERR'
}
`)
	if err == nil && strings.Contains(out, "SMB1=") {
		if strings.Contains(out, "SMB1=False") || strings.Contains(out, "SMB1=false") {
			return true, false, "SMBv1 协议已禁用"
		}
		if strings.Contains(out, "SMB1=True") || strings.Contains(out, "SMB1=true") {
			return false, false, "SMBv1 协议仍启用"
		}
	}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, smb1KeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, true, "SMBv1 状态未知"
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("SMB1")
	if err != nil {
		// Missing SMB1 value often means feature default (may still be on on older builds).
		return false, true, "SMBv1 注册表未设置（状态未知）"
	}
	if v == 0 {
		return true, false, "SMBv1 注册表已禁用"
	}
	return false, false, "SMBv1 注册表仍启用"
}

// DisableSMBv1 turns off SMB1 protocol for the SMB server (and best-effort optional feature).
func DisableSMBv1() error {
	out, err := syscmd.RunPS(`
$ErrorActionPreference='Stop'
try {
  Set-SmbServerConfiguration -EnableSMB1Protocol $false -Force -ErrorAction Stop
  Write-Output 'OK'
} catch {
  Write-Output ('ERR:' + $_.Exception.Message)
  exit 1
}
`)
	psOK := err == nil && strings.Contains(out, "OK")
	psMsg := strings.TrimSpace(out)
	if psMsg == "" && err != nil {
		psMsg = err.Error()
	}

	// Registry fallback / reinforcement.
	k, kerr := registry.OpenKey(registry.LOCAL_MACHINE, smb1KeyPath, registry.SET_VALUE)
	if kerr == nil {
		_ = k.SetDWordValue("SMB1", 0)
		k.Close()
	}

	// Best-effort disable optional feature (may require reboot; ignore failure).
	_, _ = syscmd.RunPS(`
$ErrorActionPreference='SilentlyContinue'
Disable-WindowsOptionalFeature -Online -FeatureName SMB1Protocol -NoRestart -ErrorAction SilentlyContinue | Out-Null
`)

	disabled, unknown, detail := smb1State()
	if disabled {
		return nil
	}
	if !psOK {
		return fmt.Errorf("禁用 SMBv1 失败: %s；当前: %s", psMsg, detail)
	}
	if unknown {
		return fmt.Errorf("已提交禁用 SMBv1，但无法复查（%s）。可能需重启后生效", detail)
	}
	return fmt.Errorf("已提交禁用 SMBv1，但复查仍启用（可能被组策略覆盖）")
}

func winrmState() (hardened, unknown bool, detail string) {
	svcDisabled, svcStopped, svcOK := serviceDisabledOrStopped(winRMService)
	block5985 := hasBlockRulePort(winRMBlockV4, 5985)
	block5986 := hasBlockRulePort(winRMBlockV6, 5986)

	parts := []string{}
	if svcOK {
		if svcDisabled {
			parts = append(parts, "服务=已禁用")
		} else if svcStopped {
			parts = append(parts, "服务=已停止")
		} else {
			parts = append(parts, "服务=运行中")
		}
	} else {
		parts = append(parts, "服务=未知")
	}
	if block5985 && block5986 {
		parts = append(parts, "防火墙=已拦 5985/5986")
	} else if block5985 || block5986 {
		parts = append(parts, "防火墙=部分拦截")
	} else {
		parts = append(parts, "防火墙=未拦截")
	}

	detail = strings.Join(parts, " · ")
	if !svcOK {
		return false, true, detail
	}
	// Hardened when service is disabled and both WinRM ports are blocked.
	hardened = svcDisabled && block5985 && block5986
	return hardened, false, detail
}

// HardenWinRM disables the WinRM service and blocks inbound 5985/5986.
func HardenWinRM() error {
	var errs []string

	_, _ = syscmd.Run("sc", "stop", winRMService)
	out, err := syscmd.Run("sc", "config", winRMService, "start=", "disabled")
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		// Chinese locale may need different spacing; try alternate.
		out2, err2 := syscmd.Run("sc", "config", winRMService, "start=disabled")
		if err2 != nil {
			msg2 := strings.TrimSpace(out2)
			if msg2 == "" {
				msg2 = err2.Error()
			}
			errs = append(errs, "服务: "+msg+"; "+msg2)
		}
	}

	if err := ensureBlockTCP(winRMBlockV4, 5985); err != nil {
		errs = append(errs, "5985: "+err.Error())
	}
	if err := ensureBlockTCP(winRMBlockV6, 5986); err != nil {
		errs = append(errs, "5986: "+err.Error())
	}

	hardened, unknown, detail := winrmState()
	if hardened {
		return nil
	}
	if len(errs) > 0 {
		return fmt.Errorf("限制 WinRM 失败: %s；当前: %s", strings.Join(errs, "; "), detail)
	}
	if unknown {
		return fmt.Errorf("已提交限制 WinRM，但无法复查: %s", detail)
	}
	return fmt.Errorf("已提交限制 WinRM，但复查未完全生效: %s", detail)
}

func ensureBlockTCP(name string, port uint32) error {
	portStr := strconv.FormatUint(uint64(port), 10)
	ps := fmt.Sprintf(`
$ErrorActionPreference='Stop'
$name='%s'
$port=%s
$r = Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue
if ($r) {
  Set-NetFirewallRule -Name $name -Direction Inbound -Action Block -Enabled True -Profile Any -ErrorAction Stop | Out-Null
  $r | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort $port -ErrorAction Stop | Out-Null
} else {
  New-NetFirewallRule -DisplayName $name -Name $name -Direction Inbound -Action Block -Protocol TCP -LocalPort $port -Profile Any -Enabled True -Description 'WinToolbox block WinRM port' | Out-Null
}
Write-Output 'OK'
`, name, portStr)
	out, err := syscmd.RunPS(ps)
	if err == nil && strings.Contains(out, "OK") {
		return nil
	}
	psMsg := strings.TrimSpace(out)
	if psMsg == "" && err != nil {
		psMsg = err.Error()
	}
	_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	out2, err2 := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+name,
		"dir=in",
		"action=block",
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
		return fmt.Errorf("PowerShell: %s; netsh: %s", psMsg, msg2)
	}
	return nil
}

func hasBlockRule(name string) bool {
	return hasBlockRulePort(name, 0)
}

func hasBlockRulePort(name string, port uint32) bool {
	portCheck := ""
	if port > 0 {
		portCheck = fmt.Sprintf(`
$pf = $r | Get-NetFirewallPortFilter
$local = [string]$pf.LocalPort
$want = '%d'
$ok = $false
foreach ($p in ($local -split ',')) {
  if ($p.Trim() -eq $want) { $ok = $true }
}
if (-not $ok) { Write-Output 'NO'; exit 0 }
Write-Output ("ENABLED=" + $r.Enabled + ";ACTION=" + $r.Action + ";PORT=" + $local)
`, port)
	} else {
		portCheck = `
Write-Output ("ENABLED=" + $r.Enabled + ";ACTION=" + $r.Action)
`
	}
	ps := fmt.Sprintf(`
$r=Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue
if(-not $r){Write-Output 'NO'; exit 0}
%s
`, name, portCheck)
	out, err := syscmd.RunPSQuick(ps)
	if err == nil && !strings.Contains(out, "NO") {
		enabled := strings.Contains(out, "ENABLED=True") || strings.Contains(out, "ENABLED=true")
		blocked := strings.Contains(out, "ACTION=Block") || strings.Contains(out, "ACTION=4") ||
			strings.Contains(strings.ToLower(out), "block")
		if !(enabled && blocked) {
			return false
		}
		if port > 0 {
			return portTokenExact(out, "PORT=", strconv.FormatUint(uint64(port), 10))
		}
		return true
	}

	out, err = syscmd.RunQuick("netsh", "advfirewall", "firewall", "show", "rule", "name="+name, "verbose")
	if err != nil || !strings.Contains(out, name) {
		return false
	}
	if !(strings.Contains(strings.ToLower(out), "block") || strings.Contains(out, "阻止")) {
		return false
	}
	if !netshRuleEnabled(out) {
		return false
	}
	if port > 0 {
		return netshRulePortMatches(out, strconv.FormatUint(uint64(port), 10))
	}
	return true
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
	return false
}

func portTokenExact(line, prefix, portStr string) bool {
	idx := strings.Index(line, prefix+portStr)
	if idx < 0 {
		return false
	}
	end := idx + len(prefix) + len(portStr)
	if end < len(line) {
		c := line[end]
		if c >= '0' && c <= '9' {
			return false
		}
	}
	return true
}

func netshRulePortMatches(out, portStr string) bool {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(lower, "localport") && !strings.Contains(line, "本地端口") {
			continue
		}
		want, err := strconv.ParseUint(portStr, 10, 32)
		if err != nil {
			continue
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
	}
	return false
}

func serviceDisabledOrStopped(name string) (disabled, stopped, ok bool) {
	out, err := syscmd.RunQuick("sc", "qc", name)
	if err != nil {
		return false, false, false
	}
	lower := strings.ToLower(out)
	disabled = strings.Contains(lower, "disabled") || strings.Contains(out, "已禁用")
	out2, err2 := syscmd.RunQuick("sc", "query", name)
	if err2 != nil {
		return disabled, false, true
	}
	lower2 := strings.ToLower(out2)
	stopped = strings.Contains(lower2, "stopped") || strings.Contains(out2, "已停止")
	return disabled, stopped, true
}

func anonymousState() (ok, unknown bool, detail string) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, lsaKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, true, "无法读取 LSA 匿名限制"
	}
	defer k.Close()

	ra, raOK := readDword(k, "RestrictAnonymous")
	ras, rasOK := readDword(k, "RestrictAnonymousSAM")

	parts := []string{}
	if raOK {
		parts = append(parts, fmt.Sprintf("RestrictAnonymous=%d", ra))
	} else {
		parts = append(parts, "RestrictAnonymous=未设置")
	}
	if rasOK {
		parts = append(parts, fmt.Sprintf("RestrictAnonymousSAM=%d", ras))
	} else {
		parts = append(parts, "RestrictAnonymousSAM=未设置")
	}
	detail = strings.Join(parts, " · ")

	// Hardened when RestrictAnonymous >= 1 and RestrictAnonymousSAM == 1.
	if raOK && rasOK && ra >= 1 && ras >= 1 {
		return true, false, detail
	}
	if !raOK && !rasOK {
		return false, true, detail
	}
	return false, false, detail
}

func readDword(k registry.Key, name string) (uint64, bool) {
	v, _, err := k.GetIntegerValue(name)
	if err != nil {
		return 0, false
	}
	return v, true
}

// RestrictAnonymous sets RestrictAnonymous=1 and RestrictAnonymousSAM=1.
func RestrictAnonymous() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, lsaKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开 LSA 注册表失败: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("RestrictAnonymous", 1); err != nil {
		return fmt.Errorf("写入 RestrictAnonymous 失败: %w", err)
	}
	if err := k.SetDWordValue("RestrictAnonymousSAM", 1); err != nil {
		return fmt.Errorf("写入 RestrictAnonymousSAM 失败: %w", err)
	}

	ok, unknown, detail := anonymousState()
	if ok {
		return nil
	}
	if unknown {
		return fmt.Errorf("已写入匿名限制，但无法复查: %s", detail)
	}
	return fmt.Errorf("已写入匿名限制，但复查未完全生效: %s", detail)
}
