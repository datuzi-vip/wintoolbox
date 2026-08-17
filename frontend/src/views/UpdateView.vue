<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: Object, default: null },
  busy: { type: Boolean, default: false },
})

defineEmits(['disable', 'enable'])

const disabled = computed(() => !!props.status?.updateDisabled)
const canDisable = computed(() => !props.busy && !disabled.value)
const canEnable = computed(() => !props.busy && disabled.value)
</script>

<template>
  <div>
    <h2 class="page-title">系统更新</h2>
    <p class="page-desc">一键暂停或恢复 Windows Update 相关策略与服务</p>

    <el-card shadow="never" header="Windows Update" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">状态</span>
          <el-tag :type="disabled ? 'warning' : 'success'" effect="dark">
            {{ disabled ? '已关闭' : '运行中' }}
          </el-tag>
        </div>
        <p class="wt-detail">{{ status?.updateDetail || '—' }}</p>
      </div>
      <div class="wt-actions">
        <el-button type="danger" :disabled="!canDisable" @click="$emit('disable')">
          关闭更新
        </el-button>
        <el-button type="success" :disabled="!canEnable" @click="$emit('enable')">
          恢复更新
        </el-button>
      </div>
    </el-card>
  </div>
</template>
