<script setup>
import { computed } from 'vue'

const descLabelStyle = {
  width: '84px',
  minWidth: '84px',
  maxWidth: '84px',
  whiteSpace: 'nowrap',
}

const props = defineProps({
  overview: { type: Object, default: null },
  detailLoading: { type: Boolean, default: false },
})

const actType = computed(() => {
  const s = props.overview?.activationStatus
  if (s === '已激活') return 'success'
  if (s === '检测中…' || props.detailLoading) return 'info'
  if (s === '获取失败' || s === '获取超时') return 'danger'
  if (s === '未知' || s === '—') return 'info'
  if (s?.includes('宽限期') || s?.includes('通知')) return 'warning'
  return 'danger'
})

function memText() {
  const o = props.overview
  if (!o?.memoryTotalGB) return '—'
  const used = Math.max(0, o.memoryTotalGB - (o.memoryAvailGB || 0))
  const pct = Math.round((used / o.memoryTotalGB) * 100)
  return `已用 ${used.toFixed(1)} / 共 ${o.memoryTotalGB.toFixed(1)} GB（${pct}%）`
}
</script>

<template>
  <div>
    <h2 class="page-title">本机概览</h2>
    <p class="page-desc">快速查看这台计算机的系统与硬件信息</p>

    <el-card shadow="never" header="系统" class="wt-card">
      <el-descriptions :column="1" border size="small" class="wt-desc" :label-style="descLabelStyle">
        <el-descriptions-item label="主机名">{{ overview?.hostname || '—' }}</el-descriptions-item>
        <el-descriptions-item label="架构">{{ overview?.arch || '—' }}</el-descriptions-item>
        <el-descriptions-item label="操作系统">
          {{ overview?.osName || '—' }}
          <template v-if="overview?.osBuild">（Build {{ overview.osBuild }}）</template>
        </el-descriptions-item>
        <el-descriptions-item label="激活状态">
          <el-tag :type="actType" size="small">{{ overview?.activationStatus || '—' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="分辨率">{{ overview?.resolution || '—' }}</el-descriptions-item>
        <el-descriptions-item label="IP 地址">{{ (overview?.ips || []).join(', ') || '—' }}</el-descriptions-item>
        <el-descriptions-item label="磁盘">{{ (overview?.disks || []).join('  ·  ') || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card shadow="never" header="硬件" class="wt-card" v-loading="detailLoading">
      <el-descriptions :column="1" border size="small" class="wt-desc" :label-style="descLabelStyle">
        <el-descriptions-item label="制造商">{{ overview?.manufacturer || '—' }}</el-descriptions-item>
        <el-descriptions-item label="型号">{{ overview?.model || '—' }}</el-descriptions-item>
        <el-descriptions-item label="主板">{{ overview?.board || '—' }}</el-descriptions-item>
        <el-descriptions-item label="BIOS">{{ overview?.bios || '—' }}</el-descriptions-item>
        <el-descriptions-item label="处理器">
          {{ overview?.cpu || '—' }}
          <template v-if="overview?.cpuCores">（{{ overview.cpuCores }} 逻辑处理器）</template>
        </el-descriptions-item>
        <el-descriptions-item label="内存">{{ memText() }}</el-descriptions-item>
        <el-descriptions-item label="内存条">{{ overview?.memoryModules || '—' }}</el-descriptions-item>
        <el-descriptions-item label="硬盘">{{ (overview?.physicalDisks || []).join('  ·  ') || '—' }}</el-descriptions-item>
        <el-descriptions-item label="显卡">{{ (overview?.gpus || []).join('  ·  ') || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>
