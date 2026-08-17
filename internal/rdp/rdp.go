package rdp

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/port"
	"wintoolbox/internal/win/syscmd"
)

const (
	rdpKeyPath = `SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp`
	tsKeyPath  = `SYSTEM\CurrentControlSet\Control\Terminal Server`
	fwRuleName = "WinToolbox-RDP"
)

// Status holds current Remote Desktop settings.
type Status struct {
	Enabled bool
	Port    uint32
}

// GetStatus reads whether RDP is enabled and the listening port.
func GetStatus() (Status, error) {
	port, err := GetPort()
	if err != nil {
		return Status{}, err
	}
	enabled, err := IsEnabled()
	if err != nil {
		return Status{}, err
	}
	return Status{Enabled: enabled, Port: port}, nil
}

// GetPort returns the current RDP TCP port.
func GetPort() (uint32, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, rdpKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return 0, fmt.Errorf("读取 RDP 端口失败: %w", err)
	}
	defer k.Close()

	port, _, err := k.GetIntegerValue("PortNumber")
	if err != nil {
		return 0, fmt.Errorf("读取 PortNumber 失败: %w", err)
	}
	return uint32(port), nil
}

// IsEnabled reports whether Remote Desktop connections are allowed.
func IsEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, tsKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("读取远程桌面开关失败: %w", err)
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("fDenyTSConnections")
	if err != nil {
		return false, fmt.Errorf("读取 fDenyTSConnections 失败: %w", err)
	}
	return v == 0, nil
}

func setDenyTSConnections(deny uint32) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, tsKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开远程桌面注册表失败: %w", err)
	}
	defer k.Close()
	if err := k.SetDWordValue("fDenyTSConnections", deny); err != nil {
		return fmt.Errorf("写入远程桌面开关失败: %w", err)
	}
	return nil
}

// SetEnabled enables or disables Remote Desktop.
// Enable: firewall rule first, then registry (rollback registry if rule fails after write).
// Disable: registry first, then disable custom rule.
func SetEnabled(enabled bool) error {
	if enabled {
		p, err := GetPort()
		if err != nil {
			return fmt.Errorf("读取端口失败，未开启远程桌面: %w", err)
		}
		if err := ensureCustomRuleState(p, true); err != nil {
			return fmt.Errorf("防火墙同步失败，未开启远程桌面: %w", err)
		}
		if err := setDenyTSConnections(0); err != nil {
			_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "set", "rule",
				"name="+fwRuleName, "new", "enable=no")
			return err
		}
		scheduleBuiltinRDPGroup(true)
		return nil
	}

	if err := setDenyTSConnections(1); err != nil {
		return err
	}
	_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "set", "rule",
		"name="+fwRuleName, "new", "enable=no")
	scheduleBuiltinRDPGroup(false)
	return nil
}

// ensureCustomRuleState creates/updates WinToolbox-RDP via netsh and sets enable yes/no.
func ensureCustomRuleState(p uint32, enabled bool) error {
	if err := ValidatePort(p); err != nil {
		return err
	}
	portStr := strconv.FormatUint(uint64(p), 10)
	en := "no"
	if enabled {
		en = "yes"
	}

	_, err := syscmd.Run("netsh", "advfirewall", "firewall", "set", "rule",
		"name="+fwRuleName, "new",
		"enable="+en,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport="+portStr,
		"profile=any",
	)
	if err == nil {
		return nil
	}

	out, addErr := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+fwRuleName,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport="+portStr,
		"profile=any",
		"enable="+en,
		"description=WinToolbox RDP custom port firewall rule",
	)
	if addErr != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = addErr.Error()
		}
		return fmt.Errorf("创建防火墙规则失败: %s", msg)
	}
	return nil
}

var (
	builtinDesired atomic.Bool
	builtinSeq     atomic.Uint64
	builtinKick    = make(chan struct{}, 1)
	builtinOnce    sync.Once
)

func scheduleBuiltinRDPGroup(enabled bool) {
	builtinOnce.Do(func() {
		go builtinRDPGroupWorker()
	})
	builtinDesired.Store(enabled)
	builtinSeq.Add(1)
	select {
	case builtinKick <- struct{}{}:
	default:
	}
}

func builtinRDPGroupWorker() {
	for range builtinKick {
		for {
			seq := builtinSeq.Load()
			want := builtinDesired.Load()
			syncBuiltinRDPGroup(want)
			if builtinSeq.Load() == seq {
				break
			}
		}
	}
}

func syncBuiltinRDPGroup(enabled bool) {
	enableValue := "No"
	if enabled {
		enableValue = "Yes"
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = syscmd.RunQuick("netsh", "advfirewall", "firewall", "set", "rule",
			"group=远程桌面", "new", "enable="+enableValue)
	}()
	go func() {
		defer wg.Done()
		_, _ = syscmd.RunQuick("netsh", "advfirewall", "firewall", "set", "rule",
			"group=Remote Desktop", "new", "enable="+enableValue)
	}()
	wg.Wait()
}

func scheduleBuiltinRDPGroupPort(portStr string) {
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = syscmd.RunQuick("netsh", "advfirewall", "firewall", "set", "rule",
				"group=远程桌面", "new", "localport="+portStr)
		}()
		go func() {
			defer wg.Done()
			_, _ = syscmd.RunQuick("netsh", "advfirewall", "firewall", "set", "rule",
				"group=Remote Desktop", "new", "localport="+portStr)
		}()
		wg.Wait()
	}()
}

// SetPort changes the RDP listening port, syncs firewall rules, and restarts TermService.
// When RDP is disabled, the custom rule is updated but kept disabled (does not reopen inbound).
func SetPort(port uint32) error {
	if err := ValidatePort(port); err != nil {
		return err
	}

	oldPort, err := GetPort()
	if err != nil {
		return err
	}
	if oldPort == port {
		return fmt.Errorf("当前端口已是 %d，无需修改", port)
	}

	rdpOn, err := IsEnabled()
	if err != nil {
		return fmt.Errorf("读取远程桌面状态失败: %w", err)
	}

	if err := syncPortFirewall(port, rdpOn); err != nil {
		_ = syncPortFirewall(oldPort, rdpOn)
		return fmt.Errorf("防火墙同步失败，未修改端口（仍为 %d）: %w", oldPort, err)
	}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, rdpKeyPath, registry.SET_VALUE)
	if err != nil {
		_ = syncPortFirewall(oldPort, rdpOn)
		return fmt.Errorf("打开 RDP 注册表失败: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("PortNumber", port); err != nil {
		_ = syncPortFirewall(oldPort, rdpOn)
		return fmt.Errorf("写入 PortNumber 失败，已尝试回滚防火墙到 %d: %w", oldPort, err)
	}

	if err := restartTermService(); err != nil {
		return fmt.Errorf("端口与防火墙已更新，但重启远程桌面服务失败: %w（可稍后手动重启 TermService 或重启电脑）", err)
	}
	return nil
}

func syncPortFirewall(port uint32, rdpOn bool) error {
	if rdpOn {
		return SyncFirewall(port)
	}
	return ensureCustomRuleState(port, false)
}

// ValidatePort checks whether a port is usable for RDP.
func ValidatePort(p uint32) error {
	if err := port.ValidTCP(p); err != nil {
		return err
	}
	reserved := map[uint32]string{
		22:    "SSH",
		23:    "Telnet",
		25:    "SMTP",
		53:    "DNS",
		80:    "HTTP",
		110:   "POP3",
		135:   "RPC",
		139:   "NetBIOS",
		443:   "HTTPS",
		445:   "SMB",
		1433:  "SQL Server",
		3306:  "MySQL",
		5432:  "PostgreSQL",
		5985:  "WinRM",
		5986:  "WinRM HTTPS",
		27017: "MongoDB",
	}
	if name, ok := reserved[p]; ok {
		return fmt.Errorf("端口 %d 常被 %s 占用，建议换一个", p, name)
	}
	return nil
}

// SyncFirewall ensures the WinToolbox-RDP allow rule matches port and is enabled,
// then best-effort updates built-in Remote Desktop group rules in the background.
func SyncFirewall(port uint32) error {
	if err := ValidatePort(port); err != nil {
		return err
	}
	portStr := strconv.FormatUint(uint64(port), 10)

	if err := ensureCustomRule(portStr); err != nil {
		return err
	}
	if !firewallAllowsPort(portStr) {
		return fmt.Errorf("防火墙规则校验失败，未确认端口 %s 已放行", portStr)
	}

	scheduleBuiltinRDPGroupPort(portStr)
	return nil
}

func ensureCustomRule(portStr string) error {
	for _, name := range []string{"Toolbox-RDP", "WindowsOpsTool-RDP"} {
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	}
	_, _ = syscmd.RunPS(`Get-NetFirewallRule -DisplayName 'Toolbox-RDP','WindowsOpsTool-RDP' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`)

	psUpsert := fmt.Sprintf(`
$ErrorActionPreference='Stop'
$name='%s'
$port=%s
$r = Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue
if ($r) {
  Set-NetFirewallRule -Name $name -Direction Inbound -Action Allow -Enabled True -Profile Any -ErrorAction Stop | Out-Null
  $r | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort $port -ErrorAction Stop | Out-Null
} else {
  New-NetFirewallRule -DisplayName $name -Name $name -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any -Enabled True -Description 'WinToolbox RDP custom port firewall rule' | Out-Null
}
$check = Get-NetFirewallRule -Name $name -ErrorAction Stop
$p = $check | Get-NetFirewallPortFilter
$local = [string]$p.LocalPort
if ($local -ne $port) { throw ("port mismatch: " + $local) }
Write-Output 'OK'
`, fwRuleName, portStr)
	if out, err := syscmd.RunPS(psUpsert); err == nil && strings.Contains(out, "OK") {
		return nil
	}

	_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+fwRuleName)
	_, _ = syscmd.RunPS(fmt.Sprintf(
		`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
		fwRuleName,
	))
	out, err := syscmd.Run("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+fwRuleName,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport="+portStr,
		"profile=any",
		"enable=yes",
		"description=WinToolbox RDP custom port firewall rule",
	)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("创建防火墙规则失败: %s", msg)
	}
	return nil
}

func firewallAllowsPort(portStr string) bool {
	out, err := syscmd.RunPS(fmt.Sprintf(
		`$r=Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue; if(-not $r){Write-Output 'NO'; exit 0}; $p=$r|Get-NetFirewallPortFilter; Write-Output ("ENABLED=" + $r.Enabled + ";PORT=" + $p.LocalPort + ";ACTION=" + $r.Action)`,
		fwRuleName,
	))
	if err == nil {
		enabled := strings.Contains(out, "ENABLED=True") || strings.Contains(out, "ENABLED=true")
		actionOK := strings.Contains(out, "ACTION=Allow") || strings.Contains(out, "ACTION=2") ||
			strings.Contains(strings.ToLower(out), "allow")
		if enabled && actionOK && portTokenExact(out, "PORT=", portStr) {
			return true
		}
	}

	out, err = syscmd.Run("netsh", "advfirewall", "firewall", "show", "rule", "name="+fwRuleName, "verbose")
	if err != nil || !strings.Contains(out, fwRuleName) {
		return false
	}
	allow := strings.Contains(strings.ToLower(out), "allow") || strings.Contains(out, "允许")
	return allow && netshLocalPortExact(out, portStr) && netshRuleEnabled(out)
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

func netshLocalPortExact(out, portStr string) bool {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(lower, "localport") && !strings.Contains(line, "本地端口") {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			val := strings.TrimSpace(line[idx+1:])
			for _, part := range strings.Split(val, ",") {
				part = strings.TrimSpace(part)
				if part == portStr {
					return true
				}
			}
		}
	}
	return false
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

func restartTermService() error {
	_, _ = syscmd.Run("net", "stop", "TermService", "/y")
	out, err := syscmd.Run("net", "start", "TermService")
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(out, "已经启动") || strings.Contains(lower, "already") {
			return nil
		}
		return fmt.Errorf("%s", strings.TrimSpace(out))
	}
	return nil
}
