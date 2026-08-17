import { reactive, ref } from 'vue'
import { api } from '../api/index.js'

const detailFail = '获取失败'

export function useAppState({ appendLog }) {
  const loading = ref(false)
  const overviewDetailLoading = ref(false)
  const firewallRules = ref([])
  const timeZones = ref([])
  const rdpHistory = ref([])
  const rdpHistoryLoading = ref(false)

  let detailSeq = 0
  let historySeq = 0
  let lastWarnings = []

  const state = reactive({
    overview: null,
    accounts: [],
    rdpEnabled: false,
    rdpPort: 3389,
    rdpAvailable: true,
    updateDisabled: false,
    updateDetail: '',
    defenderDisabled: false,
    defenderDetail: '',
    firewallSummary: '',
    firewallDomain: '',
    firewallPrivate: '',
    firewallPublic: '',
    firewallAllOn: false,
    firewallAllOff: false,
    timeText: '',
    timeZone: '',
    ntpServer: '',
    warnings: [],
    lockoutDisabled: false,
    lockoutUnknown: false,
    lockoutDetail: '',
  })

  const form = reactive({
    accUser: '',
    accPass1: '',
    accPass2: '',
    rdpPort: 3389,
    fwPort: 8080,
    timeZone: '',
    ntpServer: 'time.windows.com',
    powerDelay: 60,
  })

  function applyStatus(st) {
    state.overview = st.overview
    state.accounts = st.accounts || []
    state.rdpEnabled = !!st.rdpEnabled
    state.rdpPort = st.rdpPort || 0
    state.rdpAvailable = st.rdpAvailable !== false
    state.updateDisabled = st.updateDisabled
    state.updateDetail = st.updateDetail
    state.defenderDisabled = !!st.defenderDisabled
    state.defenderDetail = st.defenderDetail || ''
    state.firewallSummary = st.firewallSummary
    state.firewallDomain = st.firewallDomain || ''
    state.firewallPrivate = st.firewallPrivate || ''
    state.firewallPublic = st.firewallPublic || ''
    state.firewallAllOn = !!st.firewallAllOn
    state.firewallAllOff = !!st.firewallAllOff
    state.timeText = st.timeText
    state.timeZone = st.timeZone
    state.ntpServer = st.ntpServer
    state.warnings = st.warnings || []
    state.lockoutDisabled = !!st.lockoutDisabled
    state.lockoutUnknown = !!st.lockoutUnknown
    state.lockoutDetail = st.lockoutDetail || ''

    // Only sync RDP form from backend when status is available.
    if (state.rdpAvailable && st.rdpPort) {
      form.rdpPort = st.rdpPort
    }
    if (st.timeZone) form.timeZone = st.timeZone
    if (st.ntpServer && st.ntpServer !== '-') form.ntpServer = st.ntpServer

    if (!form.accUser && state.accounts.length) {
      const cur = state.accounts.find((a) => a.current)
      form.accUser = cur?.name || state.accounts[0].name
    }

    const prev = lastWarnings
    lastWarnings = state.warnings.slice()
    for (const w of state.warnings) {
      if (!prev.includes(w)) appendLog('提示: ' + w)
    }
  }

  function applyOverviewDetail(d) {
    if (!state.overview) return
    state.overview = {
      ...state.overview,
      memoryModules: d.memoryModules || state.overview.memoryModules,
      physicalDisks: d.physicalDisks?.length ? d.physicalDisks : state.overview.physicalDisks,
      gpus: d.gpus?.length ? d.gpus : state.overview.gpus,
      activated: d.activated,
      activationStatus: d.activationStatus || state.overview.activationStatus,
    }
  }

  function markOverviewDetailFailed(msg) {
    if (!state.overview) return
    const fail = detailFail
    state.overview = {
      ...state.overview,
      memoryModules: state.overview.memoryModules === '检测中…' ? fail : state.overview.memoryModules,
      physicalDisks: state.overview.physicalDisks?.length ? state.overview.physicalDisks : [fail],
      gpus: state.overview.gpus?.length ? state.overview.gpus : [fail],
      activationStatus:
        state.overview.activationStatus === '检测中…' ? fail : state.overview.activationStatus,
    }
    appendLog('概览详情: ' + msg)
  }

  async function queueOverviewDetail() {
    const seq = ++detailSeq
    overviewDetailLoading.value = true
    try {
      const d = await api.getOverviewDetail()
      if (seq !== detailSeq) return
      applyOverviewDetail(d)
    } catch (e) {
      if (seq !== detailSeq) return
      markOverviewDetailFailed(e?.message || String(e))
    } finally {
      if (seq === detailSeq) overviewDetailLoading.value = false
    }
  }

  async function refresh(showLog = false, invalidate = false) {
    loading.value = true
    try {
      const st = await api.getStatus(invalidate)
      applyStatus(st)
      if (showLog) appendLog('已刷新')
      queueOverviewDetail()
    } catch (e) {
      const msg = e?.message || String(e)
      appendLog('刷新失败: ' + msg)
      ElMessage.error('刷新失败: ' + msg)
      throw e
    } finally {
      loading.value = false
    }
  }

  async function ensureFirewallRules() {
    try {
      firewallRules.value = await api.getFirewallRules()
    } catch (e) {
      ElMessage.warning('防火墙规则加载失败: ' + (e?.message || e))
    }
  }

  async function ensureRdpHistory() {
    const seq = ++historySeq
    rdpHistoryLoading.value = true
    try {
      const list = (await api.getRdpHistory()) || []
      if (seq !== historySeq) return
      rdpHistory.value = list
    } catch (e) {
      if (seq !== historySeq) return
      ElMessage.warning('RDP 连接记录加载失败: ' + (e?.message || e))
    } finally {
      if (seq === historySeq) rdpHistoryLoading.value = false
    }
  }

  async function ensureTimeZones() {
    try {
      const zones = await api.getTimeZones()
      timeZones.value = zones
    } catch (e) {
      ElMessage.warning(e?.message || '时区列表加载失败')
    }
  }

  return {
    loading,
    overviewDetailLoading,
    state,
    form,
    firewallRules,
    rdpHistory,
    rdpHistoryLoading,
    timeZones,
    refresh,
    ensureFirewallRules,
    ensureRdpHistory,
    ensureTimeZones,
  }
}
