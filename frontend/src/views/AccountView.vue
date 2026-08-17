<script setup>
import { computed } from 'vue'

const props = defineProps({
  accounts: { type: Array, default: () => [] },
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
  lockoutDisabled: { type: Boolean, default: false },
  lockoutUnknown: { type: Boolean, default: false },
  lockoutDetail: { type: String, default: '' },
})

defineEmits(['change-pass', 'set-enabled', 'set-admin', 'disable-lockout', 'enable-lockout'])

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
</script>

<template>
  <div>
    <h2 class="page-title">本地账户</h2>
    <p class="page-desc">修改密码、启用/禁用、管理员身份，以及账户锁定策略</p>

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
          :disabled="busy || (!lockoutUnknown && !lockoutDisabled)"
          @click="$emit('enable-lockout')"
        >
          一键开启锁定
        </el-button>
        <el-button
          type="danger"
          :disabled="busy || (!lockoutUnknown && lockoutDisabled)"
          @click="$emit('disable-lockout')"
        >
          一键关闭锁定
        </el-button>
      </div>
      <p class="wt-hint">
        开启：阈值=10、锁定 10 分钟、复位窗口 10 分钟。关闭：阈值=0 并解锁已锁定本地账户。域策略可能被组策略覆盖。
      </p>
    </el-card>

    <el-card shadow="never" header="账户操作" class="wt-card">
      <el-form label-width="88px" class="wt-form-row">
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
            <el-button type="primary" :disabled="busy" @click="$emit('change-pass')">修改密码</el-button>
            <el-button type="success" :disabled="busy" @click="$emit('set-enabled', true)">启用</el-button>
            <el-button type="danger" :disabled="busy || selected?.current" @click="$emit('set-enabled', false)">禁用</el-button>
            <el-button type="warning" :disabled="busy" @click="$emit('set-admin', true)">设为管理员</el-button>
            <el-button :disabled="busy || selected?.current" @click="$emit('set-admin', false)">取消管理员</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
