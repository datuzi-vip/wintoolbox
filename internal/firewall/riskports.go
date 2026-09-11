package firewall

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"wintoolbox/internal/win/syscmd"
)

const riskRulePrefix = "WinToolbox-Block-Risk-"

// Default high-risk inbound TCP ports for one-click block preset.
// Intentionally excludes 3389 so RDP management remains usable.
var riskPorts = []uint32{135, 139, 445, 5985, 5986}

// RiskPortInfo is one port in the high-risk block preset.
type RiskPortInfo struct {
	Port    uint32 `json:"port"`
	Blocked bool   `json:"blocked"`
	Name    string `json:"name"`
}

// RiskBlockStatus summarizes the high-risk port block preset.
type RiskBlockStatus struct {
	AllBlocked bool
	Partial    bool
	None       bool
	Detail     string
	Ports      []RiskPortInfo
}

func riskRuleName(port uint32) string {
	return riskRulePrefix + strconv.FormatUint(uint64(port), 10)
}

// GetRiskBlockStatus reports which preset high-risk ports are blocked by WinToolbox rules.
// Uses a single PowerShell probe for all ports to stay within LoadStatus timeout.
func GetRiskBlockStatus() RiskBlockStatus {
	blockedMap := probeRiskBlocks()
	ports := make([]RiskPortInfo, 0, len(riskPorts))
	blocked := 0
	for _, p := range riskPorts {
		b := blockedMap[p]
		if b {
			blocked++
		}
		ports = append(ports, RiskPortInfo{
			Port:    p,
			Blocked: b,
			Name:    riskRuleName(p),
		})
	}
	st := RiskBlockStatus{Ports: ports}
	switch blocked {
	case 0:
		st.None = true
		st.Detail = "高危端口预设未启用"
	case len(riskPorts):
		st.AllBlocked = true
		st.Detail = fmt.Sprintf("已拦截 %d 个高危入站端口（135/139/445/5985/5986）", blocked)
	default:
		st.Partial = true
		st.Detail = fmt.Sprintf("部分拦截（%d/%d）", blocked, len(riskPorts))
	}
	return st
}

func probeRiskBlocks() map[uint32]bool {
	out := make(map[uint32]bool, len(riskPorts))
	var names []string
	for _, p := range riskPorts {
		names = append(names, riskRuleName(p))
	}
	ps := fmt.Sprintf(`
$names=@('%s')
foreach($name in $names){
  $r=Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue
  if(-not $r){ Write-Output ($name + '=0'); continue }
  $p=$r|Get-NetFirewallPortFilter
  $en=($r.Enabled -eq $true -or [string]$r.Enabled -eq 'True')
  $blk=([string]$r.Action -eq 'Block' -or [int]$r.Action -eq 4)
  $local=[string]$p.LocalPort
  $want=$name.Substring($name.LastIndexOf('-')+1)
  $portOk=$false
  foreach($part in ($local -split ',')){
    if($part.Trim() -eq $want){ $portOk=$true }
  }
  if($en -and $blk -and $portOk){ Write-Output ($name + '=1') } else { Write-Output ($name + '=0') }
}
`, strings.Join(names, "','"))
	text, err := syscmd.RunPSQuick(ps)
	if err == nil && strings.TrimSpace(text) != "" {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			for _, p := range riskPorts {
				prefix := riskRuleName(p) + "="
				if strings.HasPrefix(line, prefix) {
					out[p] = strings.HasSuffix(line, "=1")
				}
			}
		}
		if len(out) == len(riskPorts) {
			return out
		}
	}

	// Fallback: parallel netsh probes (still bounded by RunQuick).
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(riskPorts))
	for _, p := range riskPorts {
		p := p
		go func() {
			defer wg.Done()
			ok := hasRiskBlockRuleNetsh(p)
			mu.Lock()
			out[p] = ok
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// BlockRiskPorts creates inbound block rules for the high-risk TCP port preset.
func BlockRiskPorts() error {
	var errs []string
	for _, p := range riskPorts {
		if err := ensureRiskBlock(p); err != nil {
			errs = append(errs, fmt.Sprintf("%d: %v", p, err))
		}
	}
	st := GetRiskBlockStatus()
	if st.AllBlocked {
		return nil
	}
	if len(errs) > 0 {
		return fmt.Errorf("拦截高危端口失败: %s；当前: %s", strings.Join(errs, "; "), st.Detail)
	}
	return fmt.Errorf("已提交拦截，但复查未全部生效: %s", st.Detail)
}

// UnblockRiskPorts removes WinToolbox high-risk block rules.
func UnblockRiskPorts() error {
	// Batch remove via one PS call, then verify.
	var names []string
	for _, p := range riskPorts {
		names = append(names, riskRuleName(p))
	}
	_, _ = syscmd.RunPS(fmt.Sprintf(
		`Get-NetFirewallRule -Name @('%s') -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue`,
		strings.Join(names, "','"),
	))
	for _, p := range riskPorts {
		name := riskRuleName(p)
		_, _ = syscmd.Run("netsh", "advfirewall", "firewall", "delete", "rule", "name="+name)
	}
	st := GetRiskBlockStatus()
	if st.None {
		return nil
	}
	var left []string
	for _, info := range st.Ports {
		if info.Blocked {
			left = append(left, strconv.FormatUint(uint64(info.Port), 10))
		}
	}
	return fmt.Errorf("移除高危端口拦截失败，仍存在: %s", strings.Join(left, ", "))
}

func ensureRiskBlock(port uint32) error {
	name := riskRuleName(port)
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
  New-NetFirewallRule -DisplayName $name -Name $name -Direction Inbound -Action Block -Protocol TCP -LocalPort $port -Profile Any -Enabled True -Description 'WinToolbox high-risk port block preset' | Out-Null
}
$check = Get-NetFirewallRule -Name $name -ErrorAction Stop
if ($check.Enabled -ne 'True' -and $check.Enabled -ne $true) { throw 'rule not enabled' }
if ($check.Action -ne 'Block' -and $check.Action -ne 4) { throw 'rule not blocking' }
$pf = $check | Get-NetFirewallPortFilter
$local = [string]$pf.LocalPort
$ok=$false
foreach($part in ($local -split ',')){ if($part.Trim() -eq $port){ $ok=$true } }
if(-not $ok){ throw ('port mismatch: ' + $local) }
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
		"description=WinToolbox high-risk port block preset",
	)
	if err2 != nil {
		msg2 := strings.TrimSpace(out2)
		if msg2 == "" {
			msg2 = err2.Error()
		}
		return fmt.Errorf("PowerShell: %s; netsh: %s", psMsg, msg2)
	}
	if !hasRiskBlockRuleNetsh(port) && !probeRiskBlocks()[port] {
		return fmt.Errorf("规则已提交但未校验到生效（%s）", name)
	}
	return nil
}

func hasRiskBlockRule(port uint32) bool {
	return probeRiskBlocks()[port]
}

func hasRiskBlockRuleNetsh(port uint32) bool {
	name := riskRuleName(port)
	portStr := strconv.FormatUint(uint64(port), 10)
	out, err := syscmd.RunQuick("netsh", "advfirewall", "firewall", "show", "rule", "name="+name, "verbose")
	if err != nil || !strings.Contains(out, name) {
		return false
	}
	if !(strings.Contains(strings.ToLower(out), "block") || strings.Contains(out, "阻止")) {
		return false
	}
	if !netshRuleEnabled(out) {
		return false
	}
	return netshRulePortMatches(out, portStr)
}

// RiskPortsList returns the preset port numbers for UI display.
func RiskPortsList() []uint32 {
	out := make([]uint32, len(riskPorts))
	copy(out, riskPorts)
	return out
}
