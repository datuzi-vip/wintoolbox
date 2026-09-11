<script setup>
import { computed, ref, watch } from 'vue'
import { FORM_LABEL_WIDTH, FORM_LABEL_WIDTH_WIDE } from '../constants.js'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
  lockoutDisabled: { type: Boolean, default: false },
  lockoutUnknown: { type: Boolean, default: false },
  lockoutDetail: { type: String, default: '' },
  lockoutThreshold: { type: Number, default: 10 },
  lockoutDuration: { type: Number, default: 30 },
  lockoutWindow: { type: Number, default: 30 },
  guestExists: { type: Boolean, default: false },
  guestEnabled: { type: Boolean, default: false },
  guestUnknown: { type: Boolean, default: false },
  guestDetail: { type: String, default: '' },
  autoLogonEnabled: { type: Boolean, default: false },
  autoLogonUnknown: { type: Boolean, default: false },
  autoLogonDetail: { type: String, default: '' },
  passwordMinLength: { type: Number, default: -1 },
  passwordComplexity: { type: Boolean, default: false },
  passwordComplexityUnknown: { type: Boolean, default: true },
  passwordUnknown: { type: Boolean, default: true },
  passwordPolicyDetail: { type: String, default: '' },
})

defineEmits([
  'change-pass',
  'set-enabled',
  'set-admin',
  'disable-lockout',
  'enable-lockout',
  'set-lockout-policy',
  'disable-guest',
  'disable-autologon',
  'enable-password-policy',
])

const selected = computed(() => props.accounts.find((a) => a.name === props.form.accUser))

const infoText = computed(() => {
  const a = selected.value
  if (!a) return '选择用户后操作'
  const en = a.enabledUnknown ? '状态未知' : a.enabled ? '启用' : '禁用'
  const adm = a.adminUnknown ? '权限未知' : a.admin ? '管理员' : '普通'
  const cur = a.current ? ' · 当前登录' : ''
  return `${a.name} · ${en} · ${adm}${cur}`
})

const lockoutTagType = computed(() => {
  if (props.lockoutUnknown) return 'info'
  return props.lockoutDisabled ? 'success' : 'warning'
})

const lockoutTagText = computed(() => {
  if (props.lockoutUnknown) return '未知'
  return props.lockoutDisabled ? '已关闭' : '已启用'
})

const guestTag = computed(() => {
  if (props.guestUnknown) return { text: '未知', type: 'info' }
  if (!props.guestExists) return { text: '不存在', type: 'success' }
  return props.guestEnabled
    ? { text: '已启用', type: 'warning' }
    : { text: '已禁用', type: 'success' }
})

const autoTag = computed(() => {
  if (props.autoLogonUnknown) return { text: '未知', type: 'info' }
  return props.autoLogonEnabled
    ? { text: '已启用', type: 'warning' }
    : { text: '已关闭', type: 'success' }
})

const pwdTag = computed(() => {
  if (props.passwordUnknown || props.passwordMinLength < 0) {
    return { text: '未知', type: 'info' }
  }
  if (props.passwordComplexityUnknown) {
    return { text: '部分未知', type: 'info' }
  }
  const strong = props.passwordMinLength >= 12 && props.passwordComplexity
  if (strong) return { text: '已加固', type: 'success' }
  return { text: '可加强', type: 'warning' }
})

const customThreshold = ref(10)
const customDurationMin = ref(30)
const customWindowMin = ref(30)
const customMinLen = ref(12)
const lockoutDirty = ref(false)
const passwordDirty = ref(false)

watch(
  () => [props.lockoutThreshold, props.lockoutDuration, props.lockoutWindow, props.lockoutUnknown],
  ([threshold, duration, window, unknown], prev) => {
    if (unknown) return
    if (!Number.isFinite(threshold) || threshold < 0) return
    // Keep local edits until status values change (successful apply / refresh).
    if (lockoutDirty.value) {
      const changed =
        !prev ||
        threshold !== prev[0] ||
        duration !== prev[1] ||
        window !== prev[2] ||
        unknown !== prev[3]
      if (!changed) return
      lockoutDirty.value = false
    }
    customThreshold.value = threshold
    customDurationMin.value = Number.isFinite(duration) && duration > 0 ? duration : 30
    customWindowMin.value = Number.isFinite(window) && window > 0 ? window : 30
  },
  { immediate: true },
)

watch(
  () => [props.passwordMinLength, props.passwordUnknown],
  ([n, unknown], prev) => {
    if (passwordDirty.value) {
      const changed = !prev || n !== prev[0] || unknown !== prev[1]
      if (!changed) return
      passwordDirty.value = false
    }
    if (Number.isFinite(n) && n >= 1) {
      customMinLen.value = n
      return
    }
    customMinLen.value = 12
  },
  { immediate: true },
)

function markLockoutDirty() {
  lockoutDirty.value = true
}
function markPasswordDirty() {
  passwordDirty.value = true
}

const canApplyCustomLockout = computed(() => {
  if (props.busy || props.lockoutUnknown) return false
  if (!Number.isFinite(customThreshold.value)) return false
  if (customThreshold.value < 0) return false
  if (customThreshold.value === 0) return true
  return (
    customDurationMin.value >= 1 &&
    customWindowMin.value >= 1 &&
    customDurationMin.value >= customWindowMin.value
  )
})

const canDisableGuest = computed(
  () => !props.busy && !props.guestUnknown && props.guestExists && props.guestEnabled,
)
const canDisableAutoLogon = computed(
  () => !props.busy && !props.autoLogonUnknown && props.autoLogonEnabled,
)

const hasSelected = computed(() => !!selected.value)
const canChangePass = computed(() => !props.busy && hasSelected.value)
const canSetEnabled = computed(
  () => !props.busy && hasSelected.value && !selected.value?.enabledUnknown,
)
const canSetAdmin = computed(
  () => !props.busy && hasSelected.value && !selected.value?.adminUnknown,
)
</script>

<template>
  <div>
    <h2 class="page-title">本地账户</h2>
    <p class="page-desc">修改密码、启用/禁用、管理员身份，以及账户锁定与密码策略</p>

    <el-card shadow="never" header="来宾账户 / 自动登录" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">来宾账户</span>
          <el-tag :type="guestTag.type" effect="dark">{{ guestTag.text }}</el-tag>
          <span class="wt-status-sub">{{ guestDetail || '—' }}</span>
        </div>
        <div class="wt-status-line" style="margin-top: 8px">
          <span class="wt-status-label">自动登录</span>
          <el-tag :type="autoTag.type" effect="dark">{{ autoTag.text }}</el-tag>
          <span class="wt-status-sub">{{ autoLogonDetail || '—' }}</span>
        </div>
      </div>
      <div class="wt-actions">
        <el-button type="warning" :disabled="!canDisableGuest" @click="$emit('disable-guest')">
          禁用来宾账户
        </el-button>
        <el-button type="warning" :disabled="!canDisableAutoLogon" @click="$emit('disable-autologon')">
          关闭自动登录
        </el-button>
      </div>
      <p class="wt-hint">禁用来宾可降低未授权访问面；关闭自动登录会清除 Winlogon 中的 DefaultPassword。</p>
    </el-card>

    <el-card shadow="never" header="密码策略" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">密码策略</span>
          <el-tag :type="pwdTag.type" effect="dark">{{ pwdTag.text }}</el-tag>
          <span class="wt-status-sub">{{ passwordPolicyDetail || '—' }}</span>
        </div>
      </div>
      <el-form :label-width="FORM_LABEL_WIDTH_WIDE" class="wt-form-row">
        <el-form-item label="最短长度">
          <el-input-number
            v-model="customMinLen"
            :min="1"
            :max="128"
            controls-position="right"
            :disabled="busy"
            @change="markPasswordDirty"
          />
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button
              type="primary"
              :disabled="busy"
              @click="$emit('enable-password-policy', customMinLen)"
            >
              应用密码策略
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <p class="wt-hint">
        将设置密码最短长度，并开启密码复杂度（大小写/数字/符号）。域成员可能被组策略覆盖。
        <template v-if="passwordUnknown || passwordMinLength < 0">当前策略读取未知时，下方长度为待应用目标值。</template>
      </p>
    </el-card>

    <el-card shadow="never" header="账户锁定策略" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">锁定策略</span>
          <el-tag :type="lockoutTagType" effect="dark">{{ lockoutTagText }}</el-tag>
          <span class="wt-status-sub">{{ lockoutDetail || '—' }}</span>
        </div>
      </div>
      <div class="wt-actions">
        <el-button
          type="success"
          :disabled="busy || lockoutUnknown || !lockoutDisabled"
          @click="$emit('enable-lockout')"
        >
          一键开启锁定
        </el-button>
        <el-button
          type="danger"
          :disabled="busy || lockoutUnknown || lockoutDisabled"
          @click="$emit('disable-lockout')"
        >
          一键关闭锁定
        </el-button>
      </div>

      <div style="margin-top: 14px">
        <el-form :label-width="FORM_LABEL_WIDTH_WIDE" class="wt-form-row">
          <el-form-item label="自定义阈值（次）">
            <el-input-number
              v-model="customThreshold"
              :min="0"
              :max="9999"
              controls-position="right"
              @change="markLockoutDirty"
            />
          </el-form-item>
          <el-form-item label="锁定时间（分钟）">
            <el-input-number
              v-model="customDurationMin"
              :min="1"
              :max="9999"
              controls-position="right"
              :disabled="customThreshold === 0"
              @change="markLockoutDirty"
            />
          </el-form-item>
          <el-form-item label="复位窗口（分钟）">
            <el-input-number
              v-model="customWindowMin"
              :min="1"
              :max="9999"
              controls-position="right"
              :disabled="customThreshold === 0"
              @change="markLockoutDirty"
            />
          </el-form-item>
          <el-form-item>
            <div class="wt-actions">
              <el-button
                type="primary"
                :disabled="!canApplyCustomLockout"
                @click="$emit('set-lockout-policy', customThreshold, customDurationMin, customWindowMin)"
              >
                应用自定义策略
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <p class="wt-hint">
        默认优先回填当前系统策略；若当前值不可用，则使用 10 / 30 / 30。开启：阈值=10、锁定 30 分钟、复位窗口 30 分钟。关闭：阈值=0 并解锁已锁定本地账户。域策略可能被组策略覆盖。
      </p>
    </el-card>

    <el-card shadow="never" header="账户操作" class="wt-card">
      <el-form :label-width="FORM_LABEL_WIDTH" class="wt-form-row">
        <el-form-item label="用户">
          <el-select v-model="form.accUser" placeholder="选择用户" style="width: 100%">
            <el-option
              v-for="a in accounts"
              :key="a.name"
              :label="a.name + (a.current ? ' (当前)' : '')"
              :value="a.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            v-model="form.accPass1"
            type="password"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input
            v-model="form.accPass2"
            type="password"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="当前">
          <span class="wt-status-sub">{{ infoText }}</span>
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button type="primary" :disabled="!canChangePass" @click="$emit('change-pass')">修改密码</el-button>
            <el-button type="success" :disabled="!canSetEnabled" @click="$emit('set-enabled', true)">启用</el-button>
            <el-button type="danger" :disabled="!canSetEnabled || selected?.current" @click="$emit('set-enabled', false)">禁用</el-button>
            <el-button type="warning" :disabled="!canSetAdmin" @click="$emit('set-admin', true)">设为管理员</el-button>
            <el-button :disabled="!canSetAdmin || selected?.current" @click="$emit('set-admin', false)">取消管理员</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
