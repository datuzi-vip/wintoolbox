<script setup>
defineProps({
  info: { type: Object, default: null },
  busy: { type: Boolean, default: false },
  checking: { type: Boolean, default: false },
})

defineEmits(['check', 'download', 'apply'])
</script>

<template>
  <div>
    <h2 class="page-title">软件更新</h2>
    <p class="page-desc">检测 GitHub 仓库最新 Release，有更新时自动下载并校验，确认后安装并重启</p>

    <el-card shadow="never" header="版本信息" class="wt-card">
      <div class="wt-status-block">
        <div class="wt-status-line">
          <span class="wt-status-label">当前</span>
          <el-tag size="small">{{ info?.currentVersion ? 'v' + info.currentVersion : '—' }}</el-tag>
          <template v-if="info?.latestVersion">
            <span class="wt-status-label">最新</span>
            <el-tag :type="info.hasUpdate ? 'warning' : 'success'" size="small">
              v{{ info.latestVersion }}
            </el-tag>
          </template>
          <el-tag v-if="info?.hasUpdate && info?.downloaded && info?.verified" type="success" effect="dark">
            已下载·已校验
          </el-tag>
          <el-tag v-else-if="info?.hasUpdate && info?.downloaded" type="warning" effect="dark">
            已下载·未校验
          </el-tag>
          <el-tag v-else-if="info?.hasUpdate" type="warning" effect="dark">有更新</el-tag>
          <el-tag v-else-if="info?.latestVersion && !info?.hasUpdate" type="success" effect="dark">
            已是最新
          </el-tag>
        </div>
        <p v-if="info?.assetName" class="wt-detail">安装包：{{ info.assetName }}</p>
        <p v-if="info?.assetSHA256" class="wt-detail">SHA256：{{ info.assetSHA256 }}</p>
        <p v-if="info?.downloadPath" class="wt-detail">本地路径：{{ info.downloadPath }}</p>
        <p v-if="info?.error" class="wt-detail wt-detail--err">{{ info.error }}</p>
        <p v-else-if="info?.notes" class="wt-notes">{{ info.notes }}</p>
        <p v-else class="wt-detail">点击「检查更新」从 GitHub 仓库获取最新版本信息。</p>
      </div>

      <div class="wt-actions">
        <el-button type="primary" :loading="checking" :disabled="busy" @click="$emit('check')">
          检查更新
        </el-button>
        <el-button
          type="warning"
          :disabled="busy || !info?.hasUpdate || (info?.downloaded && info?.verified)"
          @click="$emit('download')"
        >
          下载更新
        </el-button>
        <el-button
          type="success"
          :disabled="busy || !info?.downloaded || !info?.verified"
          @click="$emit('apply')"
        >
          安装并重启
        </el-button>
      </div>
    </el-card>
  </div>
</template>
