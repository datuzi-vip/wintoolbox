# WinToolbox 文档

操作说明。配图中红色数字与文中标注对应。  
**下载图为模拟示意**；软件页为 **2× 高清真实界面截图**。

## 命名规则

`序号-英文短名.md` / `序号-英文短名.png`，与侧栏功能一一对应；`90-` 为综合指南。

## 第一步：下载

https://github.com/datuzi-vip/wintoolbox/releases/latest  

拦截时点 **保留 / 保持**；SmartScreen：**更多信息 → 仍要运行**。详见 [00 · 下载](./00-download.md)。

![下载拦截示意（模拟）](./images/00-download.png)

## 功能文档

| 文档 | 说明 |
|------|------|
| [00 · 下载](./00-download.md) | 下载、拦截、校验、管理员运行 |
| [01 · 本机概览](./01-overview.md) | 系统与硬件信息 |
| [02 · 本地账户](./02-local-account.md) | 来宾/自动登录、密码策略、锁定、改密 |
| [03 · 远程桌面](./03-remote-desktop.md) | 端口、开关、NLA、连接记录 |
| [04 · 防火墙](./04-firewall.md) | 配置文件、放行、禁 ping、高危端口 |
| [05 · 安全加固](./05-security-harden.md) | SMBv1、WinRM、匿名枚举 |
| [06 · 防病毒](./06-defender.md) | Defender 实时防护开关 |
| [07 · 时间同步](./07-time-sync.md) | 时区、NTP、对时与测试 |
| [08 · 电源](./08-power.md) | 锁定 / 重启 / 关机 / 取消 |
| [09 · 系统更新](./09-windows-update.md) | Windows Update 开关 |
| [10 · 软件更新](./10-app-update.md) | 本软件检查、下载、安装 |
| [90 · 系统加固指南](./90-hardening-guide.md) | 推荐顺序：账户 → RDP → 防火墙 → 加固 → 更新 |

需**管理员权限**。域环境可能被组策略覆盖。
