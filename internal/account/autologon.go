package account

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const winlogonKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon`

// AutoLogonStatus describes Winlogon automatic logon.
type AutoLogonStatus struct {
	Enabled bool
	Unknown bool
	User    string
	Detail  string
}

// GetAutoLogonStatus reads AutoAdminLogon from Winlogon.
func GetAutoLogonStatus() AutoLogonStatus {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, winlogonKey, registry.QUERY_VALUE)
	if err != nil {
		return AutoLogonStatus{Unknown: true, Detail: "读取自动登录失败"}
	}
	defer k.Close()

	val, _, err := k.GetStringValue("AutoAdminLogon")
	if err != nil {
		// Missing value means disabled.
		return AutoLogonStatus{Enabled: false, Detail: "未启用自动登录"}
	}
	user, _, _ := k.GetStringValue("DefaultUserName")
	enabled := strings.TrimSpace(val) == "1"
	detail := "未启用自动登录"
	if enabled {
		if strings.TrimSpace(user) != "" {
			detail = "已启用 · 用户 " + user
		} else {
			detail = "已启用"
		}
	}
	return AutoLogonStatus{Enabled: enabled, User: user, Detail: detail}
}

// DisableAutoLogon turns off Winlogon automatic logon and clears DefaultPassword.
func DisableAutoLogon() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, winlogonKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开 Winlogon 注册表失败: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("AutoAdminLogon", "0"); err != nil {
		return fmt.Errorf("关闭自动登录失败: %w", err)
	}
	if err := k.DeleteValue("DefaultPassword"); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("清除 DefaultPassword 失败: %w", err)
	}

	st := GetAutoLogonStatus()
	if st.Unknown {
		return fmt.Errorf("已写入关闭，但无法复查自动登录状态")
	}
	if st.Enabled {
		return fmt.Errorf("已写入关闭，但复查仍显示自动登录已启用")
	}
	return nil
}
