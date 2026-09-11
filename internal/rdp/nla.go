package rdp

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// IsNLAEnabled reports whether Network Level Authentication is required.
func IsNLAEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, rdpKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("读取 NLA 失败: %w", err)
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("UserAuthentication")
	if err != nil {
		return false, fmt.Errorf("读取 UserAuthentication 失败: %w", err)
	}
	return v == 1, nil
}

// SetNLA enables or disables Network Level Authentication for RDP.
func SetNLA(enabled bool) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, rdpKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开 RDP 注册表失败: %w", err)
	}
	defer k.Close()

	v := uint32(0)
	if enabled {
		v = 1
	}
	if err := k.SetDWordValue("UserAuthentication", v); err != nil {
		return fmt.Errorf("写入 UserAuthentication 失败: %w", err)
	}
	got, err := IsNLAEnabled()
	if err != nil {
		return fmt.Errorf("已写入 NLA，但无法复查: %w", err)
	}
	if got != enabled {
		want := "开启"
		if !enabled {
			want = "关闭"
		}
		return fmt.Errorf("已写入 NLA %s，但复查未生效（可能被组策略覆盖）", want)
	}
	return nil
}
