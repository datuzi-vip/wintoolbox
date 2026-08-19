<script setup>
import { FORM_LABEL_WIDTH, NTP_PRESETS } from '../constants.js'

defineProps({
  status: { type: Object, default: null },
  zones: { type: Array, default: () => [] },
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
})

defineEmits(['apply-tz', 'save-ntp', 'sync', 'test-ntp'])
</script>

<template>
  <div>
    <h2 class="page-title">时间同步</h2>
    <p class="page-desc">时区、NTP 源与立即对时（打开本页会自动定位当前时区）</p>

    <el-card shadow="never" header="当前状态" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">本机时间</span>
        </div>
        <p class="wt-detail">{{ status?.timeText || '—' }}</p>
      </div>
    </el-card>

    <el-card shadow="never" header="设置" class="wt-card">
      <el-form :label-width="FORM_LABEL_WIDTH" class="wt-form-row">
        <el-form-item label="时区">
          <el-select-v2
            v-model="form.timeZone"
            :options="zones.map((z) => ({ value: z.id, label: z.label || z.id }))"
            filterable
            placeholder="选择时区"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="NTP">
          <el-input v-model="form.ntpServer" placeholder="time.windows.com" />
        </el-form-item>
        <el-form-item label="常用 NTP">
          <div class="wt-tag-row">
            <el-tag
              v-for="p in NTP_PRESETS"
              :key="p.id"
              class="wt-chip"
              effect="plain"
              @click="form.ntpServer = p.id"
            >{{ p.label }}</el-tag>
          </div>
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button type="primary" :disabled="busy" @click="$emit('apply-tz')">应用时区</el-button>
            <el-button :disabled="busy" @click="$emit('save-ntp')">保存 NTP</el-button>
            <el-button type="success" :disabled="busy" @click="$emit('sync')">立即同步</el-button>
            <el-button type="info" :disabled="busy" @click="$emit('test-ntp')">测试 NTP</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
