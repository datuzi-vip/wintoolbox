<script setup>
import {computed, defineAsyncComponent, onMounted, reactive, ref, watch} from 'vue'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import {
  Monitor,
  User,
  Connection,
  Lock,
  Clock,
  SwitchButton,
  Refresh,
  FirstAidKit,
  Download,
} from '@element-plus/icons-vue'
import {APP_VERSION as APP_VERSION_FALLBACK, MENUS} from './constants.js'
import {api} from './api/index.js'
import {useAppState} from './composables/useAppState.js'
import {useActions} from './composables/useActions.js'
import OpLog from './components/OpLog.vue'

const OverviewView = defineAsyncComponent(() => import('./views/OverviewView.vue'))
const AccountView = defineAsyncComponent(() => import('./views/AccountView.vue'))
const RdpView = defineAsyncComponent(() => import('./views/RdpView.vue'))
const FirewallView = defineAsyncComponent(() => import('./views/FirewallView.vue'))
const DefenderView = defineAsyncComponent(() => import('./views/DefenderView.vue'))
const TimeView = defineAsyncComponent(() => import('./views/TimeView.vue'))
const PowerView = defineAsyncComponent(() => import('./views/PowerView.vue'))
const UpdateView = defineAsyncComponent(() => import('./views/UpdateView.vue'))
const SelfUpdateView = defineAsyncComponent(() => import('./views/SelfUpdateView.vue'))

const ICONS = {Monitor, User, Connection, Lock, Clock, SwitchButton, Refresh, FirstAidKit, Download}

const active = ref('overview')
const busy = ref(false)
const logs = ref([])
const appUpdate = reactive({
  currentVersion: '',
  latestVersion: '',
  hasUpdate: false,
  assetName: '',
  assetURL: '',
  assetSize: 0,
  assetSHA256: '',
  notes: '',
  downloaded: false,
  downloadPath: '',
  verified: false,
  error: '',
})
const appUpdateChecking = ref(false)

function appendLog(msg) {
  const t = new Date().toLocaleTimeString('zh-CN', {hour12: false})
  logs.value.push(`[${t}] ${msg}`)
}

function applyAppUpdateInfo(info) {
  if (!info) return
  appUpdate.currentVersion = info.currentVersion || ''
  appUpdate.latestVersion = info.latestVersion || ''
  appUpdate.hasUpdate = !!info.hasUpdate
  appUpdate.assetName = info.assetName || ''
  appUpdate.assetURL = info.assetURL || ''
  appUpdate.assetSize = info.assetSize || 0
  appUpdate.assetSHA256 = info.assetSHA256 || ''
  appUpdate.notes = info.notes || ''
  appUpdate.downloaded = !!info.downloaded
  appUpdate.downloadPath = info.downloadPath || ''
  appUpdate.verified = !!info.verified
  appUpdate.error = info.error || ''
}

const {
  loading,
  overviewDetailLoading,
  state,
  form,
  firewallRules,
  timeZones,
  rdpHistory,
  rdpHistoryLoading,
  refresh,
  ensureFirewallRules,
  ensureTimeZones,
  ensureRdpHistory,
} = useAppState({appendLog})

const actions = useActions({
  busy,
  appendLog,
  refresh,
  form,
  state,
  ensureFirewallRules,
  ensureRdpHistory,
})

const status = computed(() => ({
  rdpEnabled: state.rdpEnabled,
  rdpPort: state.rdpPort,
  rdpAvailable: state.rdpAvailable,
  updateDisabled: state.updateDisabled,
  updateDetail: state.updateDetail,
  defenderDisabled: state.defenderDisabled,
  defenderDetail: state.defenderDetail,
  firewallSummary: state.firewallSummary,
  firewallDomain: state.firewallDomain,
  firewallPrivate: state.firewallPrivate,
  firewallPublic: state.firewallPublic,
  firewallAllOn: state.firewallAllOn,
  firewallAllOff: state.firewallAllOff,
  pingBlocked: state.pingBlocked,
  pingIPv4Blocked: state.pingIPv4Blocked,
  pingIPv6Blocked: state.pingIPv6Blocked,
  pingState: state.pingState,
  timeText: state.timeText,
}))

const activeMenu = computed(() => MENUS.find((m) => m.key === active.value))

watch(active, (key) => {
  if (key === 'firewall') ensureFirewallRules()
  if (key === 'time') ensureTimeZones()
  if (key === 'rdp') ensureRdpHistory()
  if (key === 'selfupdate' && !appUpdate.latestVersion && !appUpdateChecking.value) {
    onCheckAppUpdate(false)
  }
})

const APP_VERSION = ref(APP_VERSION_FALLBACK)

async function onCheckAppUpdate(showToast = true) {
  appUpdateChecking.value = true
  try {
    const info = await api.checkAppUpdate()
    applyAppUpdateInfo(info)
    if (info?.hasUpdate) {
      appendLog(`发现新版本 v${info.latestVersion}`)
      if (!info.downloaded || !info.verified) {
        const dl = await api.downloadAppUpdate()
        applyAppUpdateInfo(dl)
        appendLog(`已自动下载并校验 v${dl.latestVersion}`)
        if (showToast) ElMessage.success(`已下载并校验 v${dl.latestVersion}，可安装并重启`)
      } else if (showToast) {
        ElMessage.success(`已有校验通过的 v${info.latestVersion}，可安装并重启`)
      }
    } else if (showToast) {
      ElMessage.success('已是最新版本')
      appendLog('软件已是最新版本')
    }
  } catch (e) {
    const msg = e?.message || String(e)
    appUpdate.error = msg
    appendLog('检查更新失败: ' + msg)
    if (showToast) ElMessage.error('检查更新失败: ' + msg)
  } finally {
    appUpdateChecking.value = false
  }
}

async function onDownloadAppUpdate() {
  busy.value = true
  try {
    const info = await api.downloadAppUpdate()
    applyAppUpdateInfo(info)
    appendLog(`已下载并校验 v${info.latestVersion}`)
    ElMessage.success(info.verified ? '更新包已下载并完成 SHA256 校验' : '更新包已下载')
  } catch (e) {
    const msg = e?.message || String(e)
    appendLog('下载更新失败: ' + msg)
    ElMessage.error('下载更新失败: ' + msg)
  } finally {
    busy.value = false
  }
}

async function onApplyAppUpdate() {
  try {
    await ElMessageBox.confirm(
        `确认安装 v${appUpdate.latestVersion || ''} 并重启 WinToolbox？`,
        '安装更新',
        {type: 'warning', confirmButtonText: '安装并重启', cancelButtonText: '取消'},
    )
  } catch {
    return
  }
  busy.value = true
  try {
    await api.applyAppUpdate()
    appendLog('正在安装更新并重启…')
    ElMessage.success('正在安装更新，程序即将重启')
  } catch (e) {
    const msg = e?.message || String(e)
    appendLog('安装更新失败: ' + msg)
    ElMessage.error('安装更新失败: ' + msg)
    busy.value = false
  }
}

async function backgroundAppUpdate() {
  try {
    const info = await api.checkAppUpdate()
    applyAppUpdateInfo(info)
    if (!info?.hasUpdate) return
    appendLog(`后台发现新版本 v${info.latestVersion}，开始下载并校验`)
    if (!info.downloaded || !info.verified) {
      const dl = await api.downloadAppUpdate()
      applyAppUpdateInfo(dl)
      appendLog(`更新包已下载并校验，可在「软件更新」中安装`)
    }
  } catch (e) {
    appendLog('后台检查更新失败: ' + (e?.message || e))
  }
}

onMounted(async () => {
  appendLog('就绪')
  try {
    const info = await api.getAppInfo()
    if (info?.version) APP_VERSION.value = info.version
  } catch {
    /* keep fallback from constants.js */
  }
  try {
    await refresh(false, false, true)
  } catch {
    /* refresh already reported */
  }
  ensureTimeZones()
  backgroundAppUpdate()
})

async function onRefresh() {
  try {
    const queueDetail = active.value === 'overview'
    await refresh(true, true, queueDetail)
    if (active.value === 'firewall') await ensureFirewallRules()
    if (active.value === 'time') await ensureTimeZones()
    if (active.value === 'rdp') await ensureRdpHistory()
    if (active.value === 'selfupdate') await onCheckAppUpdate(false)
  } catch {
    /* refresh already reported */
  }
}
</script>

<template>
  <el-config-provider :locale="zhCn">
    <el-container class="app-shell">
      <el-aside width="232px" class="app-aside">
        <div class="brand">
          <img class="brand__logo" src="/logo.svg" width="40" height="40" alt="WinToolbox"/>
          <div class="brand__text">
            <div class="brand__title">WinToolbox</div>
            <div class="brand__sub">本地运维工具箱</div>
          </div>
        </div>
        <div class="brand__meta">
          <span class="brand__ver">{{ APP_VERSION }}</span>
        </div>

        <el-menu
            :default-active="active"
            class="side-menu"
            background-color="transparent"
            text-color="#c5d0dc"
            active-text-color="#ffffff"
            @select="(k) => (active = k)"
        >
          <el-menu-item v-for="m in MENUS" :key="m.key" :index="m.key">
            <el-icon>
              <component :is="ICONS[m.icon]"/>
            </el-icon>
            <span>{{ m.title }}</span>
          </el-menu-item>
        </el-menu>

        <div class="aside-foot">
          <div>Copyright © WinToolbox</div>
        </div>
      </el-aside>

      <el-container class="app-main-wrap">
        <el-header class="app-header" height="60px">
          <div class="header-left">
            <div class="header-title">{{ activeMenu?.title }}</div>
            <div class="header-desc">{{ activeMenu?.desc }}</div>
          </div>
          <el-button type="primary" round :loading="loading" @click="onRefresh">刷新</el-button>
        </el-header>

        <el-main class="app-main" v-loading="loading">
          <OverviewView
              v-if="active === 'overview'"
              :overview="state.overview"
              :detail-loading="overviewDetailLoading"
          />
          <AccountView
              v-else-if="active === 'account'"
              :accounts="state.accounts"
              :form="form"
              :busy="busy"
              :lockout-disabled="state.lockoutDisabled"
              :lockout-unknown="state.lockoutUnknown"
              :lockout-detail="state.lockoutDetail"
              :lockout-threshold="state.lockoutThreshold"
              :lockout-duration="state.lockoutDuration"
              :lockout-window="state.lockoutWindow"
              @change-pass="actions.onAccPass"
              @set-enabled="actions.onAccEnabled"
              @set-admin="actions.onAccAdmin"
              @disable-lockout="actions.onDisableLockout"
              @enable-lockout="actions.onEnableLockout"
              @set-lockout-policy="actions.onSetLockoutPolicy"
          />
          <RdpView
              v-else-if="active === 'rdp'"
              :status="status"
              :form="form"
              :busy="busy"
              :history="rdpHistory"
              :history-loading="rdpHistoryLoading"
              @save-port="actions.onRdpPort"
              @toggle="actions.onRdpToggle"
              @clear-history="actions.onRdpClearHistory"
              @clear-history-kind="actions.onRdpClearHistoryByKind"
              @delete-history="actions.onRdpDeleteHistory"
              @refresh-history="ensureRdpHistory"
          />
          <FirewallView
              v-else-if="active === 'firewall'"
              :status="status"
              :rules="firewallRules"
              :form="form"
              :busy="busy"
              @allow="actions.onFwAllow"
              @remove="actions.onFwRemove"
              @remove-row="actions.onFwRemovePort"
              @enable-all="actions.onFwEnableAll"
              @disable-all="actions.onFwDisableAll"
              @disable-ping="actions.onFwDisablePing"
              @enable-ping="actions.onFwEnablePing"
              @clear-allow-all="actions.onFwClearAllAllowRules"
          />
          <DefenderView
              v-else-if="active === 'defender'"
              :status="status"
              :busy="busy"
              @disable="actions.onDisableDefender"
              @enable="actions.onEnableDefender"
          />
          <TimeView
              v-else-if="active === 'time'"
              :status="status"
              :zones="timeZones"
              :form="form"
              :busy="busy"
              @apply-tz="actions.onApplyTZ"
              @save-ntp="actions.onSaveNTP"
              @sync="actions.onSyncNTP"
               @test-ntp="actions.onTestNTP"
          />
          <PowerView
              v-else-if="active === 'power'"
              :form="form"
              :busy="busy"
              @lock="actions.onLock"
              @restart="actions.onRestart"
              @shutdown="actions.onShutdown"
              @abort="actions.onAbortPower"
          />
          <UpdateView
              v-else-if="active === 'update'"
              :status="status"
              :busy="busy"
              @disable="actions.onDisableUpdate"
              @enable="actions.onEnableUpdate"
          />
          <SelfUpdateView
              v-else-if="active === 'selfupdate'"
              :info="appUpdate"
              :busy="busy"
              :checking="appUpdateChecking"
              @check="() => onCheckAppUpdate(true)"
              @download="onDownloadAppUpdate"
              @apply="onApplyAppUpdate"
          />
        </el-main>

        <OpLog :lines="logs" @clear="logs = []"/>
      </el-container>
    </el-container>
  </el-config-provider>
</template>

<style scoped>
.app-shell {
  height: 100vh;
  overflow: hidden;
}

.app-aside {
  background: radial-gradient(120% 80% at 0% 0%, rgba(59, 130, 246, 0.16), transparent 55%),
  linear-gradient(180deg, var(--wt-sidebar-bg-2), var(--wt-sidebar-bg));
  display: flex;
  flex-direction: column;
  border-right: 1px solid rgba(255, 255, 255, 0.06);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 22px 18px 8px;
}

.brand__logo {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  flex-shrink: 0;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.28);
}

.brand__title {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.2px;
  line-height: 1.2;
}

.brand__sub {
  margin-top: 3px;
  font-size: 12px;
  color: var(--wt-sidebar-muted);
}

.brand__meta {
  padding: 0 18px 14px;
}

.brand__ver {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  color: #dbeafe;
  background: rgba(59, 130, 246, 0.18);
  border: 1px solid rgba(147, 197, 253, 0.22);
}

.side-menu {
  border-right: none;
  flex: 1;
  padding: 4px 12px 12px;
}

.side-menu :deep(.el-menu-item) {
  border-radius: 10px;
  margin-bottom: 4px;
  height: 44px;
  transition: background 0.18s ease, color 0.18s ease;
}

.side-menu :deep(.el-menu-item.is-active) {
  background: var(--wt-sidebar-active-soft) !important;
  color: #fff !important;
  box-shadow: inset 3px 0 0 var(--wt-sidebar-active);
}

.side-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.07);
}

.aside-foot {
  padding: 12px 18px 16px;
  font-size: 11px;
  color: var(--wt-sidebar-muted);
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  line-height: 1.5;
}

.app-main-wrap {
  background: linear-gradient(180deg, #f7f9fc 0%, var(--wt-page-bg) 120px);
  min-width: 0;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 22px;
  background: rgba(255, 255, 255, 0.86);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--wt-border);
}

.header-left {
  min-width: 0;
}

.header-title {
  font-size: 17px;
  font-weight: 650;
  color: var(--wt-text-primary);
  letter-spacing: -0.01em;
}

.header-desc {
  margin-top: 2px;
  font-size: 12px;
  color: var(--wt-text-secondary);
}

.app-main {
  padding: 18px 22px 14px;
  overflow: auto;
}
</style>
