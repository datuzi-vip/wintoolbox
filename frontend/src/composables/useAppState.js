import {reactive, ref} from 'vue'
import {api} from '../api/index.js'

const detailFail = '获取失败'

export function useAppState({appendLog}) {
    const loading = ref(false)
    const overviewDetailLoading = ref(false)
    const firewallRules = ref([])
    const timeZones = ref([])
    const rdpHistory = ref([])
    const rdpHistoryLoading = ref(false)

    let detailSeq = 0
    let historySeq = 0
    let statusSeq = 0
    let rulesSeq = 0
    let lastWarnings = []

    const state = reactive({
        overview: null,
        accounts: [],
        rdpEnabled: false,
        rdpPort: 3389,
        rdpAvailable: true,
        rdpNLA: false,
        rdpNLAUnknown: false,
        updateDisabled: false,
        updateUnknown: false,
        updateDetail: '',
        defenderDisabled: false,
        defenderUnknown: false,
        defenderDetail: '',
        firewallSummary: '',
        firewallDomain: '',
        firewallPrivate: '',
        firewallPublic: '',
        firewallAllOn: false,
        firewallAllOff: false,
        pingBlocked: false,
        pingIPv4Blocked: false,
        pingIPv6Blocked: false,
        pingState: 'enabled',
        riskPortsBlocked: false,
        riskPortsPartial: false,
        riskPortsUnknown: false,
        riskPortsDetail: '',
        timeText: '',
        timeZone: '',
        timeUnknown: false,
        ntpServer: '',
        warnings: [],
        lockoutDisabled: false,
        lockoutUnknown: false,
        lockoutDetail: '',
        lockoutThreshold: -1,
        lockoutDuration: -1,
        lockoutWindow: -1,
        guestExists: false,
        guestEnabled: false,
        guestUnknown: false,
        guestDetail: '',
        autoLogonEnabled: false,
        autoLogonUnknown: false,
        autoLogonDetail: '',
        passwordMinLength: -1,
        passwordComplexity: false,
        passwordComplexityUnknown: true,
        passwordUnknown: true,
        passwordPolicyDetail: '',
        smb1Disabled: false,
        smb1Unknown: false,
        smb1Detail: '',
        winrmHardened: false,
        winrmUnknown: false,
        winrmDetail: '',
        anonymousOK: false,
        anonymousUnknown: false,
        anonymousDetail: '',
    })

    const form = reactive({
        accUser: '',
        accPass1: '',
        accPass2: '',
        rdpPort: 3389,
        fwPort: 8080,
        timeZone: '',
        ntpServer: '',
        powerDelay: 60,
    })

    function applyStatus(st) {
        const prevRdpPort = state.rdpPort
        const prevTimeZone = state.timeZone
        const prevNtp = state.ntpServer

        state.overview = st.overview
        state.accounts = st.accounts || []
        state.rdpEnabled = !!st.rdpEnabled
        state.rdpPort = st.rdpPort || 0
        state.rdpAvailable = st.rdpAvailable !== false
        state.rdpNLA = !!st.rdpNLA
        state.rdpNLAUnknown = !!st.rdpNLAUnknown
        state.updateDisabled = !!st.updateDisabled
        state.updateUnknown = !!st.updateUnknown
        state.updateDetail = st.updateDetail
        state.defenderDisabled = !!st.defenderDisabled
        state.defenderUnknown = !!st.defenderUnknown
        state.defenderDetail = st.defenderDetail || ''
        state.firewallSummary = st.firewallSummary
        state.firewallDomain = st.firewallDomain || ''
        state.firewallPrivate = st.firewallPrivate || ''
        state.firewallPublic = st.firewallPublic || ''
        state.firewallAllOn = !!st.firewallAllOn
        state.firewallAllOff = !!st.firewallAllOff
        state.pingBlocked = !!st.pingBlocked
        state.pingIPv4Blocked = !!st.pingIPv4Blocked
        state.pingIPv6Blocked = !!st.pingIPv6Blocked
        state.pingState = st.pingState || 'enabled'
        state.riskPortsBlocked = !!st.riskPortsBlocked
        state.riskPortsPartial = !!st.riskPortsPartial
        state.riskPortsUnknown = !!st.riskPortsUnknown
        state.riskPortsDetail = st.riskPortsDetail || ''
        state.timeText = st.timeText
        state.timeZone = st.timeZone
        state.timeUnknown = !!st.timeUnknown
        state.ntpServer = st.ntpServer
        state.warnings = st.warnings || []
        state.lockoutDisabled = !!st.lockoutDisabled
        state.lockoutUnknown = !!st.lockoutUnknown
        state.lockoutDetail = st.lockoutDetail || ''
        state.lockoutThreshold = Number.isFinite(st.lockoutThreshold) ? st.lockoutThreshold : -1
        state.lockoutDuration = Number.isFinite(st.lockoutDuration) ? st.lockoutDuration : -1
        state.lockoutWindow = Number.isFinite(st.lockoutWindow) ? st.lockoutWindow : -1
        state.guestExists = !!st.guestExists
        state.guestEnabled = !!st.guestEnabled
        state.guestUnknown = !!st.guestUnknown
        state.guestDetail = st.guestDetail || ''
        state.autoLogonEnabled = !!st.autoLogonEnabled
        state.autoLogonUnknown = !!st.autoLogonUnknown
        state.autoLogonDetail = st.autoLogonDetail || ''
        state.passwordMinLength = Number.isFinite(st.passwordMinLength) ? st.passwordMinLength : -1
        state.passwordComplexity = !!st.passwordComplexity
        state.passwordComplexityUnknown = !!st.passwordComplexityUnknown
        state.passwordUnknown = !!st.passwordUnknown
        state.passwordPolicyDetail = st.passwordPolicyDetail || ''
        state.smb1Disabled = !!st.smb1Disabled
        state.smb1Unknown = !!st.smb1Unknown
        state.smb1Detail = st.smb1Detail || ''
        state.winrmHardened = !!st.winrmHardened
        state.winrmUnknown = !!st.winrmUnknown
        state.winrmDetail = st.winrmDetail || ''
        state.anonymousOK = !!st.anonymousOK
        state.anonymousUnknown = !!st.anonymousUnknown
        state.anonymousDetail = st.anonymousDetail || ''

        // Don't overwrite in-progress form edits (compare against previous status).
        // Empty form fields always accept the first server value.
        if (state.rdpAvailable && st.rdpPort && (!form.rdpPort || form.rdpPort === prevRdpPort)) {
            form.rdpPort = st.rdpPort
        }
        if (st.timeZone && st.timeZone !== '-' && (!form.timeZone || form.timeZone === prevTimeZone)) {
            form.timeZone = st.timeZone
        }
        if (st.ntpServer && st.ntpServer !== '-' && (!form.ntpServer || form.ntpServer === prevNtp)) {
            form.ntpServer = st.ntpServer
        }

        if (state.accounts.length) {
            const stillThere = state.accounts.some((a) => a.name === form.accUser)
            if (!form.accUser || !stillThere) {
                const cur = state.accounts.find((a) => a.current)
                form.accUser = cur?.name || state.accounts[0].name
            }
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
            physicalDisks: Array.isArray(d.physicalDisks) ? d.physicalDisks : state.overview.physicalDisks,
            gpus: Array.isArray(d.gpus) ? d.gpus : state.overview.gpus,
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

    async function refresh(showLog = false, invalidate = false, queueDetail = false) {
        const seq = ++statusSeq
        loading.value = true
        try {
            const st = await api.getStatus(invalidate)
            if (seq !== statusSeq) return
            applyStatus(st)
            if (showLog) appendLog('已刷新')
            if (queueDetail) queueOverviewDetail()
        } catch (e) {
            if (seq !== statusSeq) return
            const msg = e?.message || String(e)
            appendLog('刷新失败: ' + msg)
            ElMessage.error('刷新失败: ' + msg)
            throw e
        } finally {
            if (seq === statusSeq) loading.value = false
        }
    }

    async function ensureFirewallRules() {
        const seq = ++rulesSeq
        try {
            const list = (await api.getFirewallRules()) || []
            if (seq !== rulesSeq) return
            firewallRules.value = list
        } catch (e) {
            if (seq !== rulesSeq) return
            firewallRules.value = []
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
            rdpHistory.value = []
        } finally {
            if (seq === historySeq) rdpHistoryLoading.value = false
        }
    }

    async function ensureTimeZones() {
        try {
            const zones = await api.getTimeZones()
            timeZones.value = zones?.length ? zones : []
        } catch (e) {
            timeZones.value = []
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
