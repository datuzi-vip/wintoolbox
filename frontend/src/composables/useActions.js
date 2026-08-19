import {api} from '../api/index.js'
import {createRunner, normalizePort, powerDelayLabel} from './useAction.js'

export function useActions({busy, appendLog, refresh, form, state, ensureFirewallRules, ensureRdpHistory}) {
    const run = createRunner({busy, appendLog, refresh})

    function normalizePowerDelay() {
        const n = Number(form.powerDelay)
        if (!Number.isFinite(n) || n < 0) return 60
        const d = Math.floor(n)
        if (d > 604800) return 604800
        return d
    }

    return {
        onAccPass: async () => {
            if (!form.accPass1 || !form.accPass2) {
                const msg = '密码不能为空'
                appendLog('失败: ' + msg)
                ElMessage.warning(msg)
                return
            }
            if (form.accPass1 !== form.accPass2) {
                const msg = '两次输入的密码不一致'
                appendLog('失败: ' + msg)
                ElMessage.warning(msg)
                return
            }
            const ok = await run(
                () => api.changeAccountPassword(form.accUser, form.accPass1, form.accPass2),
                '密码已修改',
                {confirm: `确认修改用户 [${form.accUser}] 的密码？`},
            )
            if (ok) {
                form.accPass1 = ''
                form.accPass2 = ''
            }
        },
        onAccEnabled: (enabled) => {
            const action = enabled ? '启用' : '禁用'
            run(
                () => api.setAccountEnabled(form.accUser, enabled),
                '账户已' + action,
                {confirm: `确认${action}用户 [${form.accUser}]？`},
            )
        },
        onAccAdmin: (admin) => {
            const action = admin ? '设为管理员' : '取消管理员'
            run(
                () => api.setAccountAdmin(form.accUser, admin),
                '已' + action,
                {confirm: `确认对 [${form.accUser}] ${action}？`},
            )
        },
        onDisableLockout: () =>
            run(
                () => api.disableAccountLockout(),
                '账户锁定已关闭',
                {
                    confirm: '确认关闭账户锁定策略？将锁定阈值设为 0，并解锁当前已锁定的本地账户。',
                    confirmType: 'warning',
                },
            ),
        onEnableLockout: () =>
            run(
                () => api.enableAccountLockout(),
                '账户锁定已开启',
                {
                    confirm: '确认开启账户锁定策略？将设置阈值=10、锁定时间=30 分钟、复位窗口=30 分钟。',
                    confirmType: 'warning',
                },
            ),
        onSetLockoutPolicy: async (threshold, durationMin, windowMin) => {
            const ok = await run(
                () => api.setAccountLockoutPolicy(threshold, durationMin, windowMin),
                '账户锁定策略已更新',
                {
                    confirm: `确认应用锁定策略？阈值=${threshold} · 锁定时间=${durationMin} 分钟 · 复位窗口=${windowMin} 分钟。`,
                    confirmType: 'warning',
                },
            )
            // Lockout policy propagation can be slightly delayed on some systems.
            // Do an extra status refresh to make sure UI shows the updated policy.
            if (ok) {
                try {
                    await refresh(false)
                } catch {
                    appendLog('锁定策略更新后：状态刷新失败')
                }
            }
            return ok
        },
        onRdpPort: () => {
            const port = normalizePort(form.rdpPort, 0)
            if (!port) {
                ElMessage.warning('请输入有效端口（1-65535）')
                return
            }
            form.rdpPort = port
            run(
                () => api.changeRdpPort(port),
                '端口已保存并同步防火墙',
                {confirm: `确认将远程桌面端口改为 ${port}？`},
            )
        },
        onRdpToggle: async () => {
            if (!state.rdpAvailable) {
                ElMessage.warning('远程桌面状态不可用，无法操作')
                return
            }
            const enabling = !state.rdpEnabled
            const action = enabling ? '开启' : '关闭'
            // Skip blocking full refresh: toggle itself is fast; update UI then soft-refresh.
            const ok = await run(() => api.toggleRdp(), '远程桌面已' + action, {
                confirm: `确认${action}远程桌面？`,
                skipRefresh: true,
            })
            if (!ok) return
            state.rdpEnabled = enabling
            try {
                await refresh(false)
            } catch {
                appendLog('远程桌面切换后：状态刷新失败')
            }
        },
        onRdpClearHistory: () =>
            run(() => api.clearRdpHistory(), '已清理 RDP 连接记录', {
                confirm:
                    '确认清理本机 RDP 客户端连接记录？将删除历史主机/MRU、已保存用户名提示、TERMSRV 凭据、Default.rdp / 最近 .rdp，以及位图缓存（不影响远程桌面服务开关与端口）。',
                confirmType: 'warning',
                skipRefresh: true,
            }).then((detail) => {
                // Refresh list even on failure so residual rows are visible.
                ensureRdpHistory?.()
                if (detail === false) return
                if (typeof detail === 'string' && detail && detail !== '已清理 RDP 连接记录') {
                    appendLog(detail)
                }
            }),
        onRdpClearHistoryByKind: (kind) => {
            const labelMap = {
                mru: '最近主机',
                server: '已保存主机',
                credential: '凭据',
                file: '文件',
                cache: '缓存',
            }
            const label = labelMap[kind] || kind
            const successMsg = `已清理类型：${label} 的 RDP 连接记录`
            return run(() => api.clearRdpHistoryByKind(kind), successMsg, {
                confirm: `确认清理本机 RDP 连接记录中“${label}”类型的内容？此操作不影响其它类型记录。`,
                confirmType: 'warning',
                skipRefresh: true,
            }).then((detail) => {
                ensureRdpHistory?.()
                if (detail === false) return
                if (typeof detail === 'string' && detail && detail !== successMsg) {
                    appendLog(detail)
                }
            })
        },
        onRdpDeleteHistory: (row) => {
            if (!row?.kind) return
            const label = row.host || row.detail || row.kind
            run(
                () =>
                    api.deleteRdpHistoryEntry(
                        row.kind,
                        row.host || '',
                        row.username || '',
                        row.detail || '',
                        row.sid || '',
                    ),
                `已删除记录: ${label}`,
                {
                    confirm: `确认删除该条连接记录？\n类型=${row.kind} · ${label}`,
                    confirmType: 'warning',
                    skipRefresh: true,
                },
            ).then((ok) => {
                if (ok !== false) ensureRdpHistory?.()
            })
        },
        onFwAllow: () => {
            const port = normalizePort(form.fwPort, 0)
            if (!port) {
                ElMessage.warning('请输入有效端口（1-65535）')
                return
            }
            form.fwPort = port
            run(
                () => api.allowFirewallPort(port),
                `已放行 TCP ${port}`,
                {confirm: `确认放行入站 TCP ${port}？`},
            ).then((ok) => ok && ensureFirewallRules())
        },
        onFwRemove: () => {
            const port = normalizePort(form.fwPort, 0)
            if (!port) {
                ElMessage.warning('请输入有效端口（1-65535）')
                return
            }
            form.fwPort = port
            run(
                () => api.removeFirewallPort(port),
                `已删除 TCP ${port} 规则`,
                {confirm: `确认删除 TCP ${port} 的放行规则？`},
            ).then((ok) => ok && ensureFirewallRules())
        },
        onFwRemovePort: (port) => {
            const p = normalizePort(port, 0)
            if (!p) {
                ElMessage.warning('端口无效')
                return
            }
            run(
                () => api.removeFirewallPort(p),
                `已删除 TCP ${p} 规则`,
                {confirm: `确认删除 TCP ${p} 的放行规则？`},
            ).then((ok) => ok && ensureFirewallRules())
        },
        onFwClearAllAllowRules: () =>
            run(
                () => api.clearFirewallAllowRules(),
                '已删除全部放行规则',
                {
                    confirm: '确认删除所有 WinToolbox 放行规则（WinToolbox-Allow-*）？这不会影响远程桌面开关/端口，也不会影响禁 ping 规则。',
                    confirmType: 'warning',
                },
            ).then((ok) => ok && ensureFirewallRules()),
        onFwEnableAll: () =>
            run(
                () => api.setFirewallEnabled(true),
                '防火墙已全部开启',
                {confirm: '确认开启域 / 专用 / 公用全部防火墙配置文件？'},
            ),
        onFwDisableAll: () =>
            run(
                () => api.setFirewallEnabled(false),
                '防火墙已全部关闭',
                {
                    confirm: '确认关闭全部防火墙配置文件？关闭后本机将失去网络过滤保护。',
                    confirmType: 'warning',
                },
            ),
        onFwDisablePing: () =>
            run(
                () => api.disablePing(),
                '已禁用 ping',
                {
                    confirm: '确认禁止外部通过 ping 探测本机？这会阻止入站 IPv4 / IPv6 ICMP Echo 请求。',
                    confirmType: 'warning',
                },
            ),
        onFwEnablePing: () =>
            run(
                () => api.enablePing(),
                '已恢复 ping',
                {confirm: '确认恢复本机对 ping 的响应？将删除 WinToolbox 创建的禁 ping 规则。'},
            ),
        onApplyTZ: () =>
            run(() => api.applyTimeZone(form.timeZone), '时区已更新', {confirm: '确认修改时区？'}),
        onSaveNTP: () =>
            run(
                () => api.saveNTPServer(form.ntpServer),
                'NTP 已保存',
                {confirm: `确认保存 NTP 服务器为 ${form.ntpServer}？`},
            ),
        onSyncNTP: () => run(() => api.syncNTP(), '时间已同步', {confirm: '确认立即同步网络时间？'}),
        onTestNTP: async () => {
            const detail = await run(() => api.testNTPServer(form.ntpServer), null, {skipRefresh: true})
            if (typeof detail === 'string' && detail) {
                appendLog('NTP 测试: ' + detail)
                ElMessage.success('NTP 测试完成')
            }
            return detail
        },
        onLock: () => run(() => api.lockPC(), '已锁定'),
        onRestart: () => {
            const d = normalizePowerDelay()
            const when = powerDelayLabel(d)
            run(
                () => api.restartPC(d),
                `已计划${when}重启`,
                {
                    confirm: `确认${when}重启这台计算机？`,
                    confirmType: 'warning',
                    skipRefresh: true,
                },
            )
        },
        onShutdown: () => {
            const d = normalizePowerDelay()
            const when = powerDelayLabel(d)
            run(
                () => api.shutdownPC(d),
                `已计划${when}关机`,
                {
                    confirm: `确认${when}关闭这台计算机？`,
                    confirmType: 'error',
                    skipRefresh: true,
                },
            )
        },
        onAbortPower: () => run(() => api.abortPower(), '已取消关机/重启', {skipRefresh: true}),
        onDisableUpdate: () =>
            run(
                () => api.disableUpdate(),
                '更新已关闭',
                {confirm: '确认关闭系统更新？关闭后将不再自动获取安全补丁。'},
            ),
        onEnableUpdate: () =>
            run(() => api.enableUpdate(), '更新已恢复', {confirm: '确认恢复系统更新？'}),
        onDisableDefender: () =>
            run(
                () => api.disableDefender(),
                '实时防护已关闭',
                {
                    confirm: '确认关闭 Windows Defender 实时防护？本机将暂时失去实时扫描保护。',
                    confirmType: 'warning',
                },
            ),
        onEnableDefender: () =>
            run(() => api.enableDefender(), '实时防护已恢复', {confirm: '确认恢复 Windows Defender 实时防护？'}),
    }
}
