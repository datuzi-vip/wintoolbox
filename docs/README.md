# WinToolbox 文档

面向 Windows 本地运维与安全加固的操作说明。红色圆点数字与配图标注一一对应。

## 第一步（所有文档通用）

请先下载本软件：

- **最新版下载**：https://github.com/datuzi-vip/wintoolbox/releases/latest  
- 详细说明（含拦截处理）：[下载 WinToolbox](./download.md)

浏览器拦截下载时，请点 **保留 / 保持**（Keep）；运行时若出现 SmartScreen，选 **更多信息 → 仍要运行**。

![下载拦截时点保留](./images/00-download-keep.svg)

| 文档 | 说明 |
|------|------|
| [下载 WinToolbox](./download.md) | 下载链接、拦截时点保留、管理员运行 |
| [Windows 系统加固指南](./windows-hardening.md) | 账户、RDP、防火墙、安全加固推荐顺序 |
| [修改远程桌面端口](./change-rdp-port.md) | 改端口并同步防火墙、开启 NLA |
| [关闭 / 恢复系统更新](./disable-windows-update.md) | Windows Update 一键开关 |
| [修改本地账户密码](./change-password.md) | 改密、密码策略与锁定策略 |

> 运行 WinToolbox 需要**管理员权限**。域加入机器上的策略可能被组策略覆盖。
