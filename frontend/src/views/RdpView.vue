<script setup>
defineProps({
  status: { type: Object, default: null },
  form: { type: Object, required: true },
  busy: { type: Boolean, default: false },
  history: { type: Array, default: () => [] },
  historyLoading: { type: Boolean, default: false },
})

defineEmits(['save-port', 'toggle', 'clear-history', 'delete-history', 'refresh-history'])

const kindLabel = {
  mru: '最近主机',
  server: '已保存主机',
  credential: '凭据',
  file: '文件',
  cache: '缓存',
}

const kindTag = {
  mru: 'primary',
  server: 'success',
  credential: 'warning',
  file: 'info',
  cache: 'danger',
}

function labelKind(kind) {
  return kindLabel[kind] || kind || '—'
}

function tagKind(kind) {
  return kindTag[kind] || 'info'
}
</script>

<template>
  <div>
    <h2 class="page-title">远程桌面</h2>
    <p class="page-desc">开关与端口（更改端口会同步防火墙规则）；可查看并清理本机 mstsc 连接记录</p>

    <el-alert
      v-if="status && status.rdpAvailable === false"
      type="warning"
      :closable="false"
      show-icon
      title="无法读取远程桌面状态（可能未安装远程桌面服务或权限不足）"
      class="wt-card"
    />

    <el-card shadow="never" header="服务状态" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">远程桌面</span>
          <template v-if="status?.rdpAvailable !== false">
            <el-tag :type="status?.rdpEnabled ? 'success' : 'info'" effect="dark">
              {{ status?.rdpEnabled ? '已开启' : '已关闭' }}
            </el-tag>
            <span class="wt-status-sub">端口 {{ status?.rdpPort || '—' }}</span>
          </template>
          <el-tag v-else type="danger" effect="dark">不可用</el-tag>
        </div>
      </div>

      <el-form label-width="88px" class="wt-form-row">
        <el-form-item label="端口">
          <el-input-number
            v-model="form.rdpPort"
            :min="1"
            :max="65535"
            controls-position="right"
            :disabled="busy || status?.rdpAvailable === false"
          />
        </el-form-item>
        <el-form-item>
          <div class="wt-actions">
            <el-button
              type="primary"
              :disabled="busy || status?.rdpAvailable === false"
              @click="$emit('save-port')"
            >
              保存端口
            </el-button>
            <el-button
              type="warning"
              :disabled="busy || status?.rdpAvailable === false"
              @click="$emit('toggle')"
            >
              {{ status?.rdpEnabled ? '关闭' : '开启' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" header="连接记录" class="wt-card" v-loading="historyLoading">
      <p class="wt-detail" style="margin-top: 0">
        下列为本地远程桌面客户端留下的历史主机、用户名提示、TERMSRV 凭据、相关文件与位图缓存。清理不影响远程桌面服务开关与端口。
      </p>
      <div class="wt-actions wt-actions--mt">
        <el-button :disabled="busy || historyLoading" @click="$emit('refresh-history')">刷新列表</el-button>
        <el-button
          type="danger"
          plain
          :disabled="busy || historyLoading || !history.length"
          @click="$emit('clear-history')"
        >
          清理全部记录
        </el-button>
        <span class="wt-count">共 {{ history.length }} 条</span>
      </div>
      <el-table
        :data="history"
        stripe
        empty-text="暂无连接记录"
        highlight-current-row
        style="width: 100%; margin-top: 12px"
      >
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag :type="tagKind(row.kind)" size="small">{{ labelKind(row.kind) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="host" label="主机 / 名称" min-width="140" show-overflow-tooltip />
        <el-table-column prop="username" label="用户名" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.username || '—' }}</template>
        </el-table-column>
        <el-table-column prop="source" label="来源" min-width="110" show-overflow-tooltip />
        <el-table-column prop="detail" label="详情" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="danger"
              :disabled="busy || historyLoading"
              @click="$emit('delete-history', row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
