export function createRunner({ busy, appendLog, refresh }) {
  return async function run(fn, successMsg, { confirm, confirmType = 'warning', skipRefresh } = {}) {
    if (busy.value) {
      appendLog('上一操作仍在进行，请稍候…')
      ElMessage.info('上一操作仍在进行，请稍候')
      return false
    }
    // Hold busy across confirm + action to prevent stacked dangerous ops.
    busy.value = true
    try {
      if (confirm) {
        try {
          await ElMessageBox.confirm(confirm, '请确认', {
            type: confirmType,
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            closeOnClickModal: false,
            closeOnPressEscape: true,
            distinguishCancelAndClose: true,
          })
        } catch {
          return false
        }
      }
      const result = await fn()
      if (successMsg) {
        appendLog(successMsg)
        ElMessage.success(successMsg)
      }
      if (!skipRefresh) {
        try {
          await refresh(false)
        } catch (e) {
          appendLog('刷新失败: ' + (e?.message || e))
          ElMessage.warning('操作成功，但刷新状态失败')
        }
      }
      return result == null ? true : result
    } catch (e) {
      const msg = e?.message || String(e)
      appendLog('失败: ' + msg)
      ElMessage.error(msg)
      return false
    } finally {
      busy.value = false
    }
  }
}

export function powerDelayLabel(seconds) {
  const n = Number(seconds)
  if (!Number.isFinite(n) || n < 0) return '60 秒后'
  if (n === 0) return '立即'
  return `${n} 秒后`
}

export function normalizePort(value, fallback = 0) {
  const n = Number(value)
  if (!Number.isFinite(n)) return fallback
  const p = Math.floor(n)
  if (p < 1 || p > 65535) return fallback
  return p
}
