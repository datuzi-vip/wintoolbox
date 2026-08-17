package update

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"

	"wintoolbox/internal/win/syscmd"
)

var updateServices = []string{
	"wuauserv",
	"UsoSvc",
	"WaaSMedicSvc",
	"uhssvc",
	"DoSvc",
}

// defaultStartType is used when restoring Windows Update services without a snapshot.
var defaultStartType = map[string]string{
	"wuauserv":     "auto",
	"UsoSvc":       "auto",
	"WaaSMedicSvc": "demand",
	"uhssvc":       "demand",
	"DoSvc":        "auto",
}

const (
	auKeyPath     = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`
	wuKeyPath     = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`
	backupKeyPath = `SOFTWARE\WinToolbox\UpdateBackup`
)

// Status describes whether automatic updates appear disabled.
type Status struct {
	Disabled bool
	Detail   string
}

// GetStatus inspects policy registry and key update services.
func GetStatus() Status {
	noAuto, hasPolicy := readNoAutoUpdate()
	types := make([]string, len(updateServices))
	var wg sync.WaitGroup
	wg.Add(len(updateServices))
	for i, name := range updateServices {
		i, name := i, name
		go func() {
			defer wg.Done()
			types[i] = serviceStartType(name)
		}()
	}
	wg.Wait()

	disabledCount := 0
	presentCount := 0
	parts := make([]string, 0, len(updateServices))
	for i, name := range updateServices {
		t := types[i]
		if t == "n/a" {
			continue
		}
		presentCount++
		parts = append(parts, fmt.Sprintf("%s=%s", name, t))
		if t == "disabled" {
			disabledCount++
		}
	}

	policyOff := hasPolicy && noAuto
	// Treat as disabled when policy is off, or core update services are disabled
	// (so the UI "恢复" button stays available even if only services were flipped).
	coreOff := false
	for i, name := range updateServices {
		if name != "wuauserv" && name != "UsoSvc" {
			continue
		}
		if types[i] == "disabled" {
			coreOff = true
			break
		}
	}
	majorityOff := presentCount > 0 && disabledCount*2 >= presentCount
	disabled := policyOff || coreOff || majorityOff

	detail := fmt.Sprintf("策略关闭=%v · 相关服务已禁用 %d/%d", policyOff, disabledCount, presentCount)
	if len(parts) > 0 {
		detail += "（" + strings.Join(parts, ", ") + "）"
	}
	return Status{Disabled: disabled, Detail: detail}
}

// Disable turns off Windows automatic updates via policy, then best-effort service stops.
// Previous policy/service start values are snapshotted once for Enable() restore.
func Disable() error {
	_ = saveUpdateSnapshotIfNeeded()

	if err := setUpdatePolicyDisabled(); err != nil {
		return fmt.Errorf("写入更新策略失败: %w", err)
	}

	for _, name := range updateServices {
		_, _ = syscmd.Run("sc", "stop", name)
		out, err := syscmd.Run("sc", "config", name, "start=", "disabled")
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = err.Error()
			}
			if serviceMissing(msg) || accessDenied(msg) {
				_ = setServiceStartDWORD(name, 4)
				continue
			}
			_ = setServiceStartDWORD(name, 4)
		}
	}

	setUpdateTasks(false)
	return nil
}

// Enable restores Windows Update from the Disable() snapshot when available;
// otherwise clears WinToolbox policy keys and applies default service start types.
func Enable() error {
	restored, err := restoreUpdateSnapshot()
	if err != nil {
		return fmt.Errorf("恢复更新快照失败: %w", err)
	}
	if !restored {
		if err := clearUpdatePolicy(); err != nil {
			return fmt.Errorf("清除更新限制策略失败: %w", err)
		}
		for _, name := range updateServices {
			startType := defaultStartType[name]
			if startType == "" {
				startType = "demand"
			}
			out, err := syscmd.Run("sc", "config", name, "start=", startType)
			if err != nil {
				msg := strings.TrimSpace(out)
				if msg == "" {
					msg = err.Error()
				}
				if serviceMissing(msg) {
					continue
				}
				dword := uint32(3)
				if startType == "auto" {
					dword = 2
				}
				_ = setServiceStartDWORD(name, dword)
			}
		}
	}

	for _, name := range []string{"wuauserv", "UsoSvc", "DoSvc"} {
		_, _ = syscmd.Run("sc", "start", name)
	}

	setUpdateTasks(true)
	return nil
}

func saveUpdateSnapshotIfNeeded() error {
	if backupExists() {
		return nil
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, backupKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if err := k.SetDWordValue("Saved", 1); err != nil {
		return err
	}

	noAuto, hasNoAuto := readNoAutoUpdate()
	if hasNoAuto {
		_ = k.SetDWordValue("HadNoAutoUpdate", 1)
		v := uint32(0)
		if noAuto {
			v = 1
		}
		_ = k.SetDWordValue("NoAutoUpdate", v)
	} else {
		_ = k.SetDWordValue("HadNoAutoUpdate", 0)
	}

	if au, ok := readAUOptions(); ok {
		_ = k.SetDWordValue("HadAUOptions", 1)
		_ = k.SetDWordValue("AUOptions", au)
	} else {
		_ = k.SetDWordValue("HadAUOptions", 0)
	}

	for _, name := range updateServices {
		if start, ok := readServiceStartDWORD(name); ok {
			_ = k.SetDWordValue("Svc_"+name, start)
			_ = k.SetDWordValue("HadSvc_"+name, 1)
		} else {
			_ = k.SetDWordValue("HadSvc_"+name, 0)
		}
	}
	return nil
}

func backupExists() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, backupKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("Saved")
	return err == nil && v == 1
}

func restoreUpdateSnapshot() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, backupKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}

	saved, _, err := k.GetIntegerValue("Saved")
	if err != nil || saved != 1 {
		k.Close()
		return false, nil
	}

	hadNoAuto, _, _ := k.GetIntegerValue("HadNoAutoUpdate")
	var noAuto uint64
	var noAutoErr error
	if hadNoAuto == 1 {
		noAuto, _, noAutoErr = k.GetIntegerValue("NoAutoUpdate")
	}
	hadAU, _, _ := k.GetIntegerValue("HadAUOptions")
	var au uint64
	var auErr error
	if hadAU == 1 {
		au, _, auErr = k.GetIntegerValue("AUOptions")
	}

	type svcSnap struct {
		name string
		had  bool
		start uint64
	}
	svcs := make([]svcSnap, 0, len(updateServices))
	for _, name := range updateServices {
		had, _, _ := k.GetIntegerValue("HadSvc_" + name)
		s := svcSnap{name: name, had: had == 1}
		if s.had {
			s.start, _, _ = k.GetIntegerValue("Svc_" + name)
		}
		svcs = append(svcs, s)
	}
	k.Close()

	if hadNoAuto == 1 {
		if noAutoErr != nil {
			return false, noAutoErr
		}
		if err := writeAUDword("NoAutoUpdate", uint32(noAuto)); err != nil {
			return false, err
		}
	} else {
		_ = deleteAUValue("NoAutoUpdate")
	}

	if hadAU == 1 {
		if auErr != nil {
			return false, auErr
		}
		if err := writeAUDword("AUOptions", uint32(au)); err != nil {
			return false, err
		}
	} else {
		_ = deleteAUValue("AUOptions")
	}

	for _, s := range svcs {
		if !s.had {
			continue
		}
		startType := dwordToScStart(uint32(s.start))
		out, err := syscmd.Run("sc", "config", s.name, "start=", startType)
		if err != nil {
			msg := strings.TrimSpace(out)
			if serviceMissing(msg) || serviceMissing(err.Error()) {
				continue
			}
			_ = setServiceStartDWORD(s.name, uint32(s.start))
		}
	}

	_ = clearBackup()
	return true, nil
}

func clearBackup() error {
	return registry.DeleteKey(registry.LOCAL_MACHINE, backupKeyPath)
}

func dwordToScStart(v uint32) string {
	switch v {
	case 2:
		return "auto"
	case 3:
		return "demand"
	case 4:
		return "disabled"
	default:
		return "demand"
	}
}

func writeAUDword(name string, value uint32) error {
	wuKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, wuKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	wuKey.Close()
	auKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, auKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer auKey.Close()
	return auKey.SetDWordValue(name, value)
}

func deleteAUValue(name string) error {
	auKey, err := registry.OpenKey(registry.LOCAL_MACHINE, auKeyPath, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer auKey.Close()
	if err := auKey.DeleteValue(name); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

func readAUOptions() (uint32, bool) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, auKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AUOptions")
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func readServiceStartDWORD(service string) (uint32, bool) {
	path := `SYSTEM\CurrentControlSet\Services\` + service
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("Start")
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func setServiceStartDWORD(service string, start uint32) error {
	path := `SYSTEM\CurrentControlSet\Services\` + service
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue("Start", start)
}

func setUpdatePolicyDisabled() error {
	wuKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, wuKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	wuKey.Close()

	auKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, auKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer auKey.Close()

	if err := auKey.SetDWordValue("NoAutoUpdate", 1); err != nil {
		return err
	}
	if err := auKey.SetDWordValue("AUOptions", 1); err != nil {
		return err
	}
	return nil
}

func clearUpdatePolicy() error {
	auKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, auKeyPath, registry.SET_VALUE)
	if err != nil {
		if _, err2 := registry.OpenKey(registry.LOCAL_MACHINE, auKeyPath, registry.QUERY_VALUE); err2 != nil {
			return nil
		}
		return err
	}
	defer auKey.Close()
	if err := auKey.DeleteValue("NoAutoUpdate"); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("清除 NoAutoUpdate 失败: %w", err)
	}
	if err := auKey.DeleteValue("AUOptions"); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("清除 AUOptions 失败: %w", err)
	}
	return nil
}

func setUpdateTasks(enable bool) {
	flag := "/Disable"
	if enable {
		flag = "/Enable"
	}
	tasks := []string{
		`\Microsoft\Windows\WindowsUpdate\Scheduled Start`,
		`\Microsoft\Windows\WindowsUpdate\sih`,
		`\Microsoft\Windows\UpdateOrchestrator\Schedule Scan`,
		`\Microsoft\Windows\UpdateOrchestrator\UpdateModelTask`,
		`\Microsoft\Windows\UpdateOrchestrator\USO_UxBroker`,
		`\Microsoft\Windows\UpdateOrchestrator\Backup Scan`,
	}
	for _, task := range tasks {
		_, _ = syscmd.Run("schtasks", "/Change", "/TN", task, flag)
	}
}

func serviceMissing(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "1060") ||
		strings.Contains(msg, "指定的服务未安装") ||
		strings.Contains(lower, "does not exist")
}

func accessDenied(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "access is denied") ||
		strings.Contains(msg, "拒绝访问") ||
		strings.Contains(msg, "FAILED 5") ||
		strings.Contains(lower, "openService failed 5")
}

func readNoAutoUpdate() (value bool, ok bool) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, auKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("NoAutoUpdate")
	if err != nil {
		return false, false
	}
	return v == 1, true
}

func serviceStartType(name string) string {
	out, err := syscmd.Run("sc", "qc", name)
	msg := strings.TrimSpace(out)
	if err != nil {
		if serviceMissing(msg) || serviceMissing(err.Error()) {
			return "n/a"
		}
		return "unknown"
	}
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "disabled") || strings.Contains(out, "已禁用"):
		return "disabled"
	case strings.Contains(lower, "auto_start") || strings.Contains(out, "自动"):
		return "auto"
	case strings.Contains(lower, "demand_start") || strings.Contains(out, "手动"):
		return "manual"
	default:
		return "unknown"
	}
}
