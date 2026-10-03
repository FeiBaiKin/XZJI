<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'

const status = ref<AdminSchemas['SystemStatus']>()
const loading = ref(false)
const failed = ref('')

async function load() {
  loading.value = true
  failed.value = ''
  try {
    const { data, response } = await api.GET('/system/status')
    if (data) status.value = data
    else failed.value = `获取系统状态失败（HTTP ${response.status}）`
  } catch {
    failed.value = '无法连接后端服务，请确认已运行 task dev:server'
  } finally {
    loading.value = false
  }
}

function formatUptime(seconds: number) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return d > 0 ? `${d} 天 ${h} 小时` : h > 0 ? `${h} 小时 ${m} 分钟` : `${m} 分钟`
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div class="card-h">
        <span>系统状态</span>
        <el-button size="small" :loading="loading" @click="load">刷新</el-button>
      </div>
    </template>
    <el-alert v-if="failed" type="error" :title="failed" show-icon :closable="false" />
    <el-descriptions v-else-if="status" :column="2" border>
      <el-descriptions-item label="站点名称">{{ status.siteName }}</el-descriptions-item>
      <el-descriptions-item label="程序版本">{{ status.version }}</el-descriptions-item>
      <el-descriptions-item label="Go 版本">{{ status.goVersion }}</el-descriptions-item>
      <el-descriptions-item label="已运行">{{ formatUptime(status.uptimeSeconds) }}</el-descriptions-item>
      <el-descriptions-item label="数据库">
        <el-tag :type="status.database.status === 'up' ? 'success' : 'danger'">
          {{ status.database.status === 'up' ? '正常' : '不可用' }}
        </el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="数据库版本">{{ status.database.version || '-' }}</el-descriptions-item>
      <el-descriptions-item label="迁移版本">{{ status.database.migration }}</el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<style scoped>
.card-h { display: flex; align-items: center; justify-content: space-between; font-weight: 600; }
</style>
