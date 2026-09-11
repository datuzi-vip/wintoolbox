## {{VERSION}}

### 更新摘要

- 新增安全加固页：禁用 SMBv1、关闭并拦截 WinRM、限制匿名枚举
- 新增来宾账户 / 自动登录 / 密码策略；账户锁定支持自定义参数
- 远程桌面支持强制 NLA；防火墙支持高危端口预设拦截
- 强化状态探测、写后复查与页面响应速度
- 新增图文文档：系统加固、改 RDP 端口、关系统更新、改密码（见仓库 `docs/`）

### 下载说明（SmartScreen）

当前发布包**尚未使用 Authenticode 代码签名**。从浏览器下载后，Windows 可能提示「Windows 已保护你的电脑」或 SmartScreen 拦截，属常见现象（未签名 + 新文件哈希无信誉），**不一定是病毒**。

若确认来自本仓库 Release：

1. 点击 **更多信息** → **仍要运行**（企业策略可能禁止此选项）
2. 用下方 SHA256 校验文件完整性后再运行

### 校验

- 文件：`{{EXE}}`
- SHA256: {{SHA256}}

PowerShell 校验示例：

```powershell
Get-FileHash .\{{EXE}} -Algorithm SHA256
# 期望：{{SHA256}}
```

### 文档

- [系统加固指南](https://github.com/datuzi-vip/wintoolbox/blob/main/docs/windows-hardening.md)
- [修改远程桌面端口](https://github.com/datuzi-vip/wintoolbox/blob/main/docs/change-rdp-port.md)
- [关闭系统更新](https://github.com/datuzi-vip/wintoolbox/blob/main/docs/disable-windows-update.md)
- [修改本地账户密码](https://github.com/datuzi-vip/wintoolbox/blob/main/docs/change-password.md)

### 资产

请将以下文件上传到 GitHub Release：

- `{{EXE}}`（主程序）
- `{{SHA256_FILE}}`（校验和，建议一并上传）

> 软件内自更新会依次尝试：GitHub asset digest → 本说明中的 SHA256 行 → `*.sha256` / `SHA256.txt` 附件。
