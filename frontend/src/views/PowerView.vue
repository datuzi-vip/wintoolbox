<script setup>
import { FORM_LABEL_WIDTH } from '../constants.js'

defineProps({
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
})

defineEmits(['lock', 'restart', 'shutdown', 'abort'])
</script>

<template>
  <div>
    <h2 class="page-title">电源</h2>
    <p class="page-desc">锁定会话，或计划重启/关机。延迟 0 秒表示立即执行；可用「取消」中止计划</p>

    <el-card shadow="never" header="操作" class="wt-card">
      <el-form :label-width="FORM_LABEL_WIDTH" class="wt-form-row">
        <el-form-item label="延迟（秒）">
          <el-input-number v-model="form.powerDelay" :min="0" :max="604800" controls-position="right" />
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button :disabled="busy" @click="$emit('lock')">锁定</el-button>
            <el-button type="warning" :disabled="busy" @click="$emit('restart')">重启</el-button>
            <el-button type="danger" :disabled="busy" @click="$emit('shutdown')">关机</el-button>
            <el-button type="primary" :disabled="busy" @click="$emit('abort')">取消关机/重启</el-button>
          </div>
        </el-form-item>
      </el-form>
      <p class="wt-hint">延迟范围 0–604800 秒（最长 7 天）。执行关机/重启前会弹出确认。</p>
    </el-card>
  </div>
</template>
