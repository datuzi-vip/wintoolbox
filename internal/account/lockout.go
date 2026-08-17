package account

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"wintoolbox/internal/win/syscmd"
)

const (
	ufLockout = 0x0010

	// DefaultEnable* is the one-click "enable lockout" policy.
	DefaultEnableThreshold = 10
	DefaultEnableDuration  = 10 // minutes
	DefaultEnableWindow    = 10 // minutes

	timeqForever = uint32(0xFFFFFFFF)
)

// USER_MODALS_INFO_3 — lockout fields are in seconds.
type userModalsInfo3 struct {
	LockoutDuration          uint32
	LockoutObservationWindow uint32
	LockoutThreshold         uint32
}

// LockoutPolicy describes local account lockout settings from `net accounts`.
type LockoutPolicy struct {
	Threshold int    // 0 = lockout disabled; -1 = unknown/unparsed
	Duration  string // e.g. "30" minutes or "Never"
	Window    string // observation window
	Disabled  bool   // Threshold == 0 (only meaningful when !Unknown)
	Unknown   bool   // parse failed / unsupported locale
	Detail    string
}

// GetLockoutPolicy reads lockout policy via NetUserModalsGet (locale-independent),
// falling back to parsing `net accounts` text when the API is unavailable.
func GetLockoutPolicy() (LockoutPolicy, error) {
	if p, err := getLockoutPolicyAPI(); err == nil {
		return p, nil
	}
	return getLockoutPolicyNetAccounts()
}

func getLockoutPolicyAPI() (LockoutPolicy, error) {
	var buf *userModalsInfo3
	r0, _, _ := procNetUserModalsGet.Call(0, 3, uintptr(unsafe.Pointer(&buf)))
	if r0 != 0 {
		return LockoutPolicy{}, fmt.Errorf("%s", mapNetError(r0))
	}
	if buf == nil {
		return LockoutPolicy{}, fmt.Errorf("NetUserModalsGet 返回空缓冲")
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))

	p := LockoutPolicy{
		Threshold: int(buf.LockoutThreshold),
		Duration:  formatLockoutMinutes(buf.LockoutDuration),
		Window:    formatLockoutMinutes(buf.LockoutObservationWindow),
	}
	p.Unknown = false
	p.Disabled = p.Threshold == 0
	if p.Disabled {
		p.Detail = "锁定策略已关闭（阈值=0）"
	} else {
		p.Detail = fmt.Sprintf("阈值=%d · 锁定时间=%s · 复位窗口=%s", p.Threshold, dashStr(p.Duration), dashStr(p.Window))
	}
	return p, nil
}

func formatLockoutMinutes(seconds uint32) string {
	if seconds == timeqForever {
		return "Never"
	}
	return strconv.Itoa(int(seconds / 60))
}

func setLockoutPolicyAPI(threshold, durationMin, windowMin uint32) error {
	info := userModalsInfo3{
		LockoutThreshold: threshold,
	}
	if durationMin == 0 && threshold == 0 {
		info.LockoutDuration = timeqForever
	} else {
		info.LockoutDuration = durationMin * 60
	}
	if windowMin == 0 && threshold == 0 {
		info.LockoutObservationWindow = timeqForever
	} else {
		info.LockoutObservationWindow = windowMin * 60
	}
	r0, _, _ := procNetUserModalsSet.Call(0, 3, uintptr(unsafe.Pointer(&info)), 0)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	return nil
}

func getLockoutPolicyNetAccounts() (LockoutPolicy, error) {
	out, err := syscmd.Run("net", "accounts")
	if err != nil {
		return LockoutPolicy{}, fmt.Errorf("读取账户策略失败: %w", err)
	}
	p := parseNetAccounts(out)
	if p.Threshold < 0 {
		p.Unknown = true
		p.Disabled = false
		p.Detail = "无法解析锁定策略（可能为不受支持的系统语言）"
		return p, nil
	}
	p.Disabled = p.Threshold == 0
	if p.Disabled {
		p.Detail = "锁定策略已关闭（阈值=0）"
	} else {
		p.Detail = fmt.Sprintf("阈值=%d · 锁定时间=%s · 复位窗口=%s", p.Threshold, dashStr(p.Duration), dashStr(p.Window))
	}
	return p, nil
}

func dashStr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}

func parseNetAccounts(out string) LockoutPolicy {
	p := LockoutPolicy{Threshold: -1, Duration: "—", Window: "—"}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		label, val := splitNetAccountsLine(line)
		labelLower := strings.ToLower(label)

		switch {
		case matchAny(labelLower, lower,
			"lockout threshold",
			"锁定阈值", "鎖定閾值", "鎖定阀值", "鎖定阈值",
		):
			if n, ok := firstInt(val); ok {
				p.Threshold = n
			} else if isNeverValue(val) {
				p.Threshold = 0
			}
		case matchAny(labelLower, lower,
			"lockout duration",
			"锁定持续时间", "鎖定持續時間", "鎖定持续时间",
		):
			p.Duration = normalizeMinuteValue(val)
		case matchAny(labelLower, lower,
			"lockout observation",
			"observation window",
			"锁定观察窗口", "鎖定觀察視窗", "鎖定观察窗口",
		):
			p.Window = normalizeMinuteValue(val)
		case matchAny(labelLower, lower,
			"reset lockout counter after",
			"复位锁定计数器", "重設鎖定計數器", "复位鎖定計數器",
		):
			if p.Window == "—" {
				p.Window = normalizeMinuteValue(val)
			}
		}
	}
	if p.Threshold < 0 {
		if q := parseNetAccountsByRoleAnchor(lines); q.Threshold >= 0 {
			return q
		}
	}
	return p
}

// parseNetAccountsByRoleAnchor recovers values when labels are mojibake:
// the three numeric fields immediately above the "Computer role" line are
// threshold / duration / observation window.
func parseNetAccountsByRoleAnchor(lines []string) LockoutPolicy {
	p := LockoutPolicy{Threshold: -1, Duration: "—", Window: "—"}
	roleIdx := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		_, val := splitNetAccountsLine(trim)
		v := strings.ToUpper(strings.TrimSpace(val))
		if v == "WORKSTATION" || v == "SERVER" || v == "PRIMARY" || v == "BACKUP" ||
			strings.EqualFold(val, "工作站") || strings.Contains(val, "服务器") || strings.Contains(val, "伺服器") {
			roleIdx = i
			break
		}
		lower := strings.ToLower(trim)
		if strings.Contains(lower, "computer role") || strings.Contains(trim, "计算机角色") || strings.Contains(trim, "電腦角色") {
			roleIdx = i
			break
		}
	}
	if roleIdx < 0 {
		return p
	}
	var nums []int
	for i := roleIdx - 1; i >= 0 && len(nums) < 3; i-- {
		trim := strings.TrimSpace(lines[i])
		if trim == "" {
			continue
		}
		_, val := splitNetAccountsLine(trim)
		if isNeverValue(val) {
			nums = append(nums, -2) // sentinel for Never on duration/window
			continue
		}
		if n, ok := firstInt(val); ok {
			nums = append(nums, n)
		}
	}
	if len(nums) < 3 {
		return p
	}
	// nums are collected bottom-up: [window, duration, threshold]
	win, dur, th := nums[0], nums[1], nums[2]
	if th < 0 {
		return p
	}
	p.Threshold = th
	if dur == -2 {
		p.Duration = "Never"
	} else {
		p.Duration = strconv.Itoa(dur)
	}
	if win == -2 {
		p.Window = "Never"
	} else {
		p.Window = strconv.Itoa(win)
	}
	return p
}

func splitNetAccountsLine(line string) (label, value string) {
	// Prefer colon / fullwidth colon separators used by net accounts.
	for _, sep := range []string{":", "："} {
		if i := strings.Index(line, sep); i >= 0 {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len(sep):])
		}
	}
	// Fallback: last whitespace-separated token as value.
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return strings.Join(fields[:len(fields)-1], " "), fields[len(fields)-1]
}

func matchAny(labelLower, fullLower string, keys ...string) bool {
	for _, k := range keys {
		k = strings.ToLower(k)
		if strings.Contains(labelLower, k) || strings.Contains(fullLower, k) {
			return true
		}
	}
	return false
}

func firstInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Scan leading integer anywhere in token stream ("10", "10 minutes", "10 分钟").
	var b strings.Builder
	started := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			started = true
			continue
		}
		if started {
			break
		}
	}
	if b.Len() == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(b.String())
	if err != nil {
		return 0, false
	}
	return n, true
}

func isNeverValue(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return lower == "never" ||
		strings.Contains(lower, "never") ||
		strings.Contains(s, "无") ||
		strings.Contains(s, "無") ||
		strings.Contains(s, "永不")
}

func normalizeMinuteValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	if isNeverValue(s) {
		return "Never"
	}
	if n, ok := firstInt(s); ok {
		return strconv.Itoa(n)
	}
	return s
}

func lastField(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// DisableLockout turns off account lockout (threshold=0) and unlocks currently locked local users.
// Prefers NetUserModalsSet; falls back to `net accounts` with retries for SAM visibility lag.
func DisableLockout() error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
		}
		if err := setLockoutPolicyAPI(0, 0, 0); err != nil {
			out, netErr := syscmd.Run("net", "accounts", "/lockoutthreshold:0")
			if netErr != nil {
				msg := strings.TrimSpace(out)
				if msg == "" {
					msg = netErr.Error()
				}
				lastErr = fmt.Errorf("关闭锁定策略失败: API=%v; net=%s", err, msg)
				continue
			}
		}

		pol, verifyErr := waitLockoutDisabled(5)
		if verifyErr != nil {
			lastErr = verifyErr
			continue
		}
		if pol.Unknown {
			unlocked, unlockErr := unlockAllLocked()
			if unlockErr != nil && unlocked == 0 {
				return fmt.Errorf("已提交关闭锁定，但无法复查且解锁账户失败: %w", unlockErr)
			}
			return nil
		}

		unlocked, unlockErr := unlockAllLocked()
		if unlockErr != nil && unlocked == 0 {
			return fmt.Errorf("锁定策略已关闭，但解锁账户失败: %w", unlockErr)
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("关闭锁定策略失败：多次重试后仍未生效")
}

func waitLockoutDisabled(tries int) (LockoutPolicy, error) {
	var last LockoutPolicy
	var lastErr error
	for i := 0; i < tries; i++ {
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		pol, err := GetLockoutPolicy()
		last = pol
		if err != nil {
			lastErr = err
			continue
		}
		if pol.Unknown {
			return pol, nil
		}
		if pol.Disabled {
			return pol, nil
		}
		lastErr = fmt.Errorf("已提交关闭锁定，但复查阈值仍为 %d", pol.Threshold)
	}
	if lastErr != nil {
		return last, lastErr
	}
	return last, fmt.Errorf("复查锁定策略失败")
}

// EnableLockout restores the default local lockout policy:
// threshold=10, duration=10 minutes, observation window=10 minutes.
func EnableLockout() error {
	var lastErr error
	th := uint32(DefaultEnableThreshold)
	dur := uint32(DefaultEnableDuration)
	win := uint32(DefaultEnableWindow)
	thStr := strconv.Itoa(DefaultEnableThreshold)
	durStr := strconv.Itoa(DefaultEnableDuration)
	winStr := strconv.Itoa(DefaultEnableWindow)

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
		}
		apiErr := setLockoutPolicyAPI(th, dur, win)
		if apiErr != nil {
			var errs []string
			errs = append(errs, "API: "+apiErr.Error())
			for _, c := range [][]string{
				{"net", "accounts", "/lockoutthreshold:" + thStr},
				{"net", "accounts", "/lockoutduration:" + durStr},
				{"net", "accounts", "/lockoutwindow:" + winStr},
			} {
				out, err := syscmd.Run(c[0], c[1:]...)
				if err != nil {
					msg := strings.TrimSpace(out)
					if msg == "" {
						msg = err.Error()
					}
					errs = append(errs, msg)
				}
			}
			if len(errs) > 1 {
				lastErr = fmt.Errorf("开启锁定策略失败: %s", strings.Join(errs, "; "))
			} else {
				lastErr = apiErr
			}
		}

		pol, polErr := waitLockoutEnabled(5)
		if polErr != nil {
			if lastErr != nil {
				lastErr = fmt.Errorf("%v; %v", lastErr, polErr)
			} else {
				lastErr = polErr
			}
			continue
		}
		_ = pol
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("开启锁定策略失败：多次重试后仍未生效")
}

func waitLockoutEnabled(tries int) (LockoutPolicy, error) {
	var last LockoutPolicy
	var lastErr error
	for i := 0; i < tries; i++ {
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		pol, err := GetLockoutPolicy()
		last = pol
		if err != nil {
			lastErr = fmt.Errorf("开启锁定后复查失败: %w", err)
			continue
		}
		if pol.Unknown {
			return pol, fmt.Errorf("已提交开启锁定，但无法解析复查结果（系统语言可能不受支持）")
		}
		if pol.Disabled || pol.Threshold < DefaultEnableThreshold {
			lastErr = fmt.Errorf("已提交开启锁定，但复查阈值仍为 %d（可能被组策略覆盖）", pol.Threshold)
			continue
		}
		dur := parseMinutesField(pol.Duration)
		win := parseMinutesField(pol.Window)
		if dur != DefaultEnableDuration || win != DefaultEnableWindow {
			lastErr = fmt.Errorf("阈值已为 %d，但锁定时间/复位窗口未达到 %d 分钟（当前 %s / %s，可能被组策略覆盖）",
				pol.Threshold, DefaultEnableDuration, dashStr(pol.Duration), dashStr(pol.Window))
			continue
		}
		return pol, nil
	}
	if lastErr != nil {
		return last, lastErr
	}
	return last, fmt.Errorf("复查锁定策略失败")
}

func parseMinutesField(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return -1
	}
	if isNeverValue(s) {
		return -1
	}
	if n, ok := firstInt(s); ok {
		return n
	}
	return -1
}

func unlockAllLocked() (int, error) {
	names, err := ListLocalUsers()
	if err != nil {
		return 0, err
	}
	var firstErr error
	n := 0
	for _, name := range names {
		locked, err := isLocked(name)
		if err != nil || !locked {
			continue
		}
		if err := clearLockout(name); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n++
	}
	return n, firstErr
}

func isLocked(username string) (bool, error) {
	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return false, err
	}
	var buf *userInfo1
	r0, _, _ := procNetUserGetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1,
		uintptr(unsafe.Pointer(&buf)),
	)
	if r0 != 0 {
		return false, fmt.Errorf("%s", mapNetError(r0))
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))
	return buf.Flags&ufLockout != 0, nil
}

func clearLockout(username string) error {
	userPtr, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	var buf *userInfo1
	r0, _, _ := procNetUserGetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1,
		uintptr(unsafe.Pointer(&buf)),
	)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))

	flags := buf.Flags &^ ufLockout
	flags |= ufScript
	payload := userInfo1008{Flags: flags}
	r0, _, _ = procNetUserSetInfo.Call(
		0,
		uintptr(unsafe.Pointer(userPtr)),
		1008,
		uintptr(unsafe.Pointer(&payload)),
		0,
	)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	return nil
}
