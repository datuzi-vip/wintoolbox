<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: Object, default: null },
  busy: { type: Boolean, default: false },
})

defineEmits(['disable', 'enable'])

const disabled = computed(() => !!props.status?.defenderDisabled)
const canDisable = computed(() => !props.busy && !disabled.value)
const canEnable = computed(() => !props.busy && disabled.value)
</script>

<template>
  <div>
    <h2 class="page-title">防病毒</h2>
    <p class="page-desc">一键关闭或恢复 Windows Defender 实时防护（若开启篡改防护可能失败）</p>

    <el-card shadow="never" header="实时防护" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">状态</span>
          <el-tag :type="disabled ? 'danger' : 'success'" effect="dark">
            {{ disabled ? '已关闭' : '运行中' }}
          </el-tag>
        </div>
        <p class="wt-detail">{{ status?.defenderDetail || '—' }}</p>
      </div>
      <div class="wt-actions">
        <el-button type="danger" :disabled="!canDisable" @click="$emit('disable')">
          关闭实时防护
        </el-button>
        <el-button type="success" :disabled="!canEnable" @click="$emit('enable')">
          恢复实时防护
        </el-button>
      </div>
    </el-card>
  </div>
</template>
