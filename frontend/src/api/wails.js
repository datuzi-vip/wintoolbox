/** Wails bind target: ui.App (see internal/ui/bind.go) */
const BIND_PKG = 'ui'
const BIND_TYPE = 'App'

export function wailsApp() {
  return window.go?.[BIND_PKG]?.[BIND_TYPE] ?? null
}

export function wailsCall(method, ...args) {
  const fn = wailsApp()?.[method]
  if (typeof fn !== 'function') {
    return Promise.reject(new Error('后端未就绪，请确认以 Wails 方式启动'))
  }
  return fn(...args)
}
