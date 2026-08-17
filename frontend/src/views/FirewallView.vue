<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: Object, default: null },
  rules: { type: Array, default: () => [] },
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
})

defineEmits(['allow', 'remove', 'remove-row', 'enable-all', 'disable-all'])

const profiles = computed(() => [
  { key: 'domain', label: '域', value: props.status?.firewallDomain || '未知' },
  { key: 'private', label: '专用', value: props.status?.firewallPrivate || '未知' },
  { key: 'public', label: '公用', value: props.status?.firewallPublic || '未知' },
])

function profileTagType(value) {
  if (value === '开') return 'success'
  if (value === '关') return 'danger'
  return 'info'
}

function profileLabel(value) {
  if (value === '开') return '已开启'
  if (value === '关') return '已关闭'
  return value || '未知'
}

const overall = computed(() => {
  if (props.status?.firewallAllOn) {
    return { text: '全部开启', type: 'success', desc: '域 / 专用 / 公用配置文件均已启用' }
  }
  if (props.status?.firewallAllOff) {
    return { text: '全部关闭', type: 'danger', desc: '域 / 专用 / 公用配置文件均已关闭' }
  }
  const vals = profiles.value.map((p) => p.value)
  if (vals.every((v) => v !== '开' && v !== '关')) {
    return { text: '状态未知', type: 'info', desc: '未能可靠读取防火墙配置文件状态' }
  }
  return { text: '部分开启', type: 'warning', desc: '部分配置文件已开启，部分已关闭或未知' }
})

const canEnableAll = computed(
  () => !props.busy && !props.status?.firewallAllOn && overall.value.text !== '状态未知',
)
const canDisableAll = computed(
  () => !props.busy && !props.status?.firewallAllOff && overall.value.text !== '状态未知',
)
</script>

<template>
  <div>
    <h2 class="page-title">防火墙</h2>
    <p class="page-desc">一键开关配置文件，并为入站 TCP 添加放行规则（规则名前缀 WinToolbox）</p>

    <el-card shadow="never" header="配置文件" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">总状态</span>
          <el-tag :type="overall.type" effect="dark">{{ overall.text }}</el-tag>
        </div>
        <p class="wt-detail">{{ overall.desc }}</p>
      </div>

      <div class="wt-stat-grid">
        <div v-for="p in profiles" :key="p.key" class="wt-stat-item">
          <span class="wt-stat-name">{{ p.label }}</span>
          <el-tag :type="profileTagType(p.value)" effect="plain" size="small">
            {{ profileLabel(p.value) }}
          </el-tag>
        </div>
      </div>

      <div class="wt-actions wt-actions--mt">
        <el-button type="success" :disabled="!canEnableAll" @click="$emit('enable-all')">
          全部开启
        </el-button>
        <el-button type="danger" :disabled="!canDisableAll" @click="$emit('disable-all')">
          全部关闭
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never" header="端口操作" class="wt-card">
      <el-form inline class="wt-form-row">
        <el-form-item label="TCP 端口">
          <el-input-number v-model="form.fwPort" :min="1" :max="65535" controls-position="right" />
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button type="success" :disabled="busy" @click="$emit('allow')">放行</el-button>
            <el-button type="danger" :disabled="busy" @click="$emit('remove')">按端口删除</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" header="已创建规则" class="wt-card">
      <el-table :data="rules" stripe empty-text="暂无规则" highlight-current-row style="width: 100%">
        <el-table-column prop="name" label="规则名称" min-width="240" />
        <el-table-column prop="port" label="端口" width="90" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '开启' : '关闭' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" :disabled="busy" @click="$emit('remove-row', row.port)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
