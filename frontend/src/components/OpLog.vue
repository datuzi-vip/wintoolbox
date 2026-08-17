<script setup>
defineProps({
  lines: { type: Array, default: () => [] },
})
defineEmits(['clear'])
</script>

<template>
  <div class="op-log">
    <div class="op-log__head">
      <span class="op-log__title">活动日志</span>
      <el-button text type="primary" size="small" @click="$emit('clear')">清空</el-button>
    </div>
    <el-scrollbar class="op-log__body" always>
      <div v-for="(line, i) in lines" :key="i" class="op-log__line">{{ line }}</div>
      <div v-if="!lines.length" class="op-log__empty">暂无记录</div>
    </el-scrollbar>
  </div>
</template>

<style scoped>
.op-log {
  border-top: 1px solid var(--wt-border);
  background: rgba(255, 255, 255, 0.92);
  padding: 10px 18px 12px;
}
.op-log__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.op-log__title {
  font-size: 12px;
  font-weight: 600;
  color: var(--wt-text-secondary);
  letter-spacing: 0.02em;
}
.op-log__body {
  height: 68px;
}
.op-log__line {
  font-size: 12px;
  color: var(--wt-text-regular);
  line-height: 1.65;
  font-family: Consolas, 'Cascadia Mono', 'Microsoft YaHei UI', monospace;
}
.op-log__empty {
  font-size: 12px;
  color: var(--wt-text-placeholder);
}
</style>
