<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: Object, default: null },
  busy: { type: Boolean, default: false },
})

defineEmits(['disable-smb1', 'harden-winrm', 'restrict-anonymous'])

function itemState(unknown, good, goodText, badText) {
  if (unknown) return { text: '未知', type: 'info' }
  if (good) return { text: goodText, type: 'success' }
  return { text: badText, type: 'warning' }
}

const smb1 = computed(() =>
  itemState(props.status?.smb1Unknown, props.status?.smb1Disabled, '已禁用', '仍启用'),
)
const winrm = computed(() =>
  itemState(props.status?.winrmUnknown, props.status?.winrmHardened, '已加固', '未加固'),
)
const anonymous = computed(() =>
  itemState(props.status?.anonymousUnknown, props.status?.anonymousOK, '已限制', '未限制'),
)

const canDisableSmb1 = computed(
  () => !props.busy && !props.status?.smb1Unknown && !props.status?.smb1Disabled,
)
const canHardenWinRM = computed(
  () => !props.busy && !props.status?.winrmUnknown && !props.status?.winrmHardened,
)
const canRestrictAnonymous = computed(
  () => !props.busy && !props.status?.anonymousUnknown && !props.status?.anonymousOK,
)
</script>

<template>
  <div>
    <h2 class="page-title">安全加固</h2>
    <p class="page-desc">针对服务器常见暴露面的一键加固：SMBv1、WinRM、匿名枚举（域策略可能覆盖）</p>

    <el-card shadow="never" header="禁用 SMBv1" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">SMBv1</span>
          <el-tag :type="smb1.type" effect="dark">{{ smb1.text }}</el-tag>
          <span class="wt-status-sub">{{ status?.smb1Detail || '—' }}</span>
        </div>
      </div>
      <div class="wt-actions">
        <el-button type="warning" :disabled="!canDisableSmb1" @click="$emit('disable-smb1')">
          禁用 SMBv1
        </el-button>
      </div>
      <p class="wt-hint">关闭 SMB 服务器的 SMBv1 协议，降低蠕虫/勒索相关风险。部分极老客户端可能无法访问共享。</p>
    </el-card>

    <el-card shadow="never" header="限制 WinRM" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">WinRM</span>
          <el-tag :type="winrm.type" effect="dark">{{ winrm.text }}</el-tag>
          <span class="wt-status-sub">{{ status?.winrmDetail || '—' }}</span>
        </div>
      </div>
      <div class="wt-actions">
        <el-button type="warning" :disabled="!canHardenWinRM" @click="$emit('harden-winrm')">
          关闭并拦截 WinRM
        </el-button>
      </div>
      <p class="wt-hint">将 WinRM 服务设为禁用，并拦截入站 TCP 5985 / 5986。若本机依赖远程 PowerShell / Ansible，请勿执行。</p>
    </el-card>

    <el-card shadow="never" header="限制匿名枚举" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">匿名访问</span>
          <el-tag :type="anonymous.type" effect="dark">{{ anonymous.text }}</el-tag>
          <span class="wt-status-sub">{{ status?.anonymousDetail || '—' }}</span>
        </div>
      </div>
      <div class="wt-actions">
        <el-button type="warning" :disabled="!canRestrictAnonymous" @click="$emit('restrict-anonymous')">
          禁匿名枚举
        </el-button>
      </div>
      <p class="wt-hint">设置 RestrictAnonymous=1 与 RestrictAnonymousSAM=1，减少匿名枚举本地账户与共享信息。</p>
    </el-card>
  </div>
</template>
