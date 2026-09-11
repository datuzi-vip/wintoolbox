# 00 · 下载 WinToolbox

所有操作文档的**第一步**。

## 下载链接

- 最新版：https://github.com/datuzi-vip/wintoolbox/releases/latest  
- 全部版本：https://github.com/datuzi-vip/wintoolbox/releases  

下载 **`WinToolbox-v*.exe`**（可选同名 `.sha256`）。

## 拦截时点「保留 / 保持」

未签名 exe **常被拦截**，不等于一定有毒。

**Edge / Chrome**：已阻止 → **…** → **保留 / 保持**（Keep）→ 必要时 **仍要保留**。  

**SmartScreen**：更多信息 → **仍要运行**。

![下载拦截示意（模拟）](./images/00-download.png)

## 校验（可选）

```powershell
Get-FileHash .\WinToolbox-v1.2.0.exe -Algorithm SHA256
```

与 Release / `.sha256` 对比。

## 运行

**以管理员身份运行**（或双击后在 UAC 允许）。

返回：[文档首页](./README.md)
