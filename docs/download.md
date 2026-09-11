# 下载 WinToolbox

所有操作文档的**第一步**都是下载并运行本软件。

## 下载链接

- 最新版（推荐）：https://github.com/datuzi-vip/wintoolbox/releases/latest  
- 发行版列表：https://github.com/datuzi-vip/wintoolbox/releases  

在 Release 页面下载 **`WinToolbox-v*.exe`**（可同时下载同名 `.sha256` 做校验）。

## 下载被拦截时（务必点「保留 / 保持」）

浏览器或 Windows 安全中心**大概率会拦截**未签名的 exe，这不等于一定是病毒（本项目当前未做 Authenticode 代码签名）。

### Microsoft Edge / Chrome

1. 下载栏出现「已阻止」「不安全」等提示时，点击 **…** / 箭头展开。  
2. 选择 **保留** / **保持**（英文界面为 **Keep**）。  
3. 若继续提示风险，再选 **仍要保留** / **Keep anyway**。

### Windows SmartScreen（双击运行时）

1. 出现「Windows 已保护你的电脑」→ 点 **更多信息**。  
2. 再点 **仍要运行**。

### 建议校验

```powershell
Get-FileHash .\WinToolbox-v1.2.0.exe -Algorithm SHA256
```

将结果与 Release 说明或 `*.sha256` 文件中的哈希对比。

## 运行

右键 **以管理员身份运行**（或双击后在 UAC 中允许）。本工具需要管理员权限才能改系统设置。
