export const APP_NAME = 'WinToolbox' // synced from version.json
export const APP_VERSION = 'v1.1' // synced from version.json

export const MENUS = [
  { key: 'overview', title: '本机概览', icon: 'Monitor', desc: '系统与硬件' },
  { key: 'account', title: '本地账户', icon: 'User', desc: '密码、权限与锁定策略' },
  { key: 'rdp', title: '远程桌面', icon: 'Connection', desc: '开关、端口与连接记录' },
  { key: 'firewall', title: '防火墙', icon: 'Lock', desc: '一键开关与端口放行' },
  { key: 'defender', title: '防病毒', icon: 'FirstAidKit', desc: 'Defender 实时防护' },
  { key: 'time', title: '时间同步', icon: 'Clock', desc: '时区与 NTP' },
  { key: 'power', title: '电源', icon: 'SwitchButton', desc: '锁定 / 重启 / 关机' },
  { key: 'update', title: '系统更新', icon: 'Refresh', desc: 'Windows Update 开关' },
  { key: 'selfupdate', title: '软件更新', icon: 'Download', desc: '检测仓库并下载安装' },
]

export const NTP_PRESETS = [
  { id: 'time.windows.com', label: 'Microsoft' },
  { id: 'ntp.aliyun.com', label: '阿里云' },
  { id: 'ntp.tencent.com', label: '腾讯云' },
  { id: 'cn.pool.ntp.org', label: 'NTP Pool' },
  { id: 'ntp.ntsc.ac.cn', label: '国家授时' },
  { id: 'time.nist.gov', label: 'NIST' },
]

export const FORM_LABEL_WIDTH = '96px'
export const FORM_LABEL_WIDTH_WIDE = '136px'
