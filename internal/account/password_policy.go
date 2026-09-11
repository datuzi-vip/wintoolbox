package account

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"wintoolbox/internal/win/syscmd"
)

const (
	DefaultMinPasswordLength = 12

	// USER_MODALS_INFO_0 time units are in seconds for ages; min length is characters.
)

// USER_MODALS_INFO_0 — password / age policy.
type userModalsInfo0 struct {
	MinPasswdLen    uint32
	MaxPasswdAge    uint32
	MinPasswdAge    uint32
	ForceLogoff     uint32
	PasswordHistLen uint32
}

// PasswordPolicy describes local password length and complexity.
type PasswordPolicy struct {
	MinLength          int
	Complexity         bool
	ComplexityUnknown  bool
	Unknown            bool
	Detail             string
}

// GetPasswordPolicy reads min length via NetUserModalsGet and complexity via secedit export.
func GetPasswordPolicy() PasswordPolicy {
	p := PasswordPolicy{MinLength: -1, ComplexityUnknown: true, Unknown: true}

	if min, err := getMinPasswordLengthAPI(); err == nil {
		p.MinLength = int(min)
		p.Unknown = false
	}

	if c, ok := getPasswordComplexitySecedit(); ok {
		p.Complexity = c
		p.ComplexityUnknown = false
		p.Unknown = false
	}

	parts := []string{}
	if p.MinLength >= 0 {
		parts = append(parts, fmt.Sprintf("最短长度=%d", p.MinLength))
	} else {
		parts = append(parts, "最短长度=未知")
	}
	if p.ComplexityUnknown {
		parts = append(parts, "复杂度=未知")
	} else if p.Complexity {
		parts = append(parts, "复杂度=已开启")
	} else {
		parts = append(parts, "复杂度=已关闭")
	}
	p.Detail = strings.Join(parts, " · ")
	return p
}

func getMinPasswordLengthAPI() (uint32, error) {
	var buf *userModalsInfo0
	r0, _, _ := procNetUserModalsGet.Call(0, 0, uintptr(unsafe.Pointer(&buf)))
	if r0 != 0 {
		return 0, fmt.Errorf("%s", mapNetError(r0))
	}
	if buf == nil {
		return 0, fmt.Errorf("NetUserModalsGet 返回空缓冲")
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))
	return buf.MinPasswdLen, nil
}

func setMinPasswordLengthAPI(n uint32) error {
	var buf *userModalsInfo0
	r0, _, _ := procNetUserModalsGet.Call(0, 0, uintptr(unsafe.Pointer(&buf)))
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	if buf == nil {
		return fmt.Errorf("NetUserModalsGet 返回空缓冲")
	}
	defer procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buf)))

	info := *buf
	info.MinPasswdLen = n
	r0, _, _ = procNetUserModalsSet.Call(0, 0, uintptr(unsafe.Pointer(&info)), 0)
	if r0 != 0 {
		return fmt.Errorf("%s", mapNetError(r0))
	}
	return nil
}

func getPasswordComplexitySecedit() (bool, bool) {
	dir, err := os.MkdirTemp("", "wintoolbox-secpol-*")
	if err != nil {
		return false, false
	}
	defer os.RemoveAll(dir)

	cfg := filepath.Join(dir, "export.inf")
	out, err := syscmd.Run("secedit", "/export", "/cfg", cfg, "/areas", "SECURITYPOLICY")
	if err != nil {
		_ = out
		return false, false
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "passwordcomplexity") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			v := strings.TrimSpace(parts[1])
			return v == "1", true
		}
	}
	return false, false
}

// EnablePasswordPolicy sets min length (default 12) and turns on password complexity.
func EnablePasswordPolicy(minLen int) error {
	if minLen < 0 || minLen > 128 {
		return fmt.Errorf("密码最短长度无效（0–128）")
	}
	if minLen == 0 {
		minLen = DefaultMinPasswordLength
	}

	var errs []string
	if err := setMinPasswordLengthAPI(uint32(minLen)); err != nil {
		out, netErr := syscmd.Run("net", "accounts", "/minpwlen:"+strconv.Itoa(minLen))
		if netErr != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = netErr.Error()
			}
			errs = append(errs, fmt.Sprintf("最短长度: API=%v; net=%s", err, msg))
		}
	}

	if err := setPasswordComplexity(true); err != nil {
		errs = append(errs, "复杂度: "+err.Error())
	}

	// Verify best-effort.
	pol := GetPasswordPolicy()
	if pol.MinLength < 0 {
		errs = append(errs, "复查最短长度失败（无法读取当前策略）")
	} else if pol.MinLength < minLen {
		errs = append(errs, fmt.Sprintf("复查最短长度仍为 %d（期望 >= %d，可能被组策略覆盖）", pol.MinLength, minLen))
	}
	if pol.ComplexityUnknown {
		errs = append(errs, "复查复杂度失败（无法读取当前策略）")
	} else if !pol.Complexity {
		errs = append(errs, "复查复杂度仍为关闭（可能被组策略覆盖）")
	}
	if len(errs) > 0 {
		return fmt.Errorf("应用密码策略部分失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SetPasswordMinLength sets only the minimum password length.
func SetPasswordMinLength(minLen int) error {
	if minLen < 0 || minLen > 128 {
		return fmt.Errorf("密码最短长度无效（0–128）")
	}
	if err := setMinPasswordLengthAPI(uint32(minLen)); err != nil {
		out, netErr := syscmd.Run("net", "accounts", "/minpwlen:"+strconv.Itoa(minLen))
		if netErr != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = netErr.Error()
			}
			return fmt.Errorf("设置密码最短长度失败: API=%v; net=%s", err, msg)
		}
	}
	return nil
}

// SetPasswordComplexity enables or disables password complexity via secedit.
func SetPasswordComplexity(enabled bool) error {
	return setPasswordComplexity(enabled)
}

func setPasswordComplexity(enabled bool) error {
	dir, err := os.MkdirTemp("", "wintoolbox-secpol-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(dir)

	flag := "0"
	if enabled {
		flag = "1"
	}
	cfg := filepath.Join(dir, "policy.inf")
	db := filepath.Join(dir, "policy.sdb")
	content := "[Unicode]\r\nUnicode=yes\r\n[System Access]\r\nPasswordComplexity = " + flag + "\r\n[Version]\r\nsignature=\"$CHICAGO$\"\r\nRevision=1\r\n"
	if err := os.WriteFile(cfg, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入策略文件失败: %w", err)
	}

	out, err := syscmd.Run("secedit", "/configure", "/db", db, "/cfg", cfg, "/areas", "SECURITYPOLICY")
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("配置密码复杂度失败: %s", msg)
	}

	// Brief wait for SAM to settle, then verify.
	time.Sleep(300 * time.Millisecond)
	c, ok := getPasswordComplexitySecedit()
	if !ok {
		return fmt.Errorf("已提交密码复杂度变更，但无法复查（secedit 导出失败）")
	}
	if c != enabled {
		want := "开启"
		if !enabled {
			want = "关闭"
		}
		return fmt.Errorf("已提交复杂度%s，但复查未生效（可能被组策略覆盖）", want)
	}
	return nil
}
