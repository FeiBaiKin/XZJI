<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()
</script>

<template>
  <el-container class="layout">
    <el-aside width="208px" class="aside">
      <div class="logo">XZJI 后台</div>
      <el-menu :default-active="route.path" router>
        <el-menu-item index="/">仪表盘</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">{{ route.meta.title }}</el-header>
      <el-main class="main">
        <RouterView />
      </el-main>
    </el-container>
  </el-container>
</template>

<style>
html, body, #app { height: 100%; margin: 0; }
body { font-family: "PingFang SC", "Microsoft YaHei", system-ui, sans-serif; background: #F4F5F7; }
</style>

<style scoped>
.layout { height: 100%; }
.aside { background: #fff; border-right: 1px solid #EEF0F3; }
.logo { height: 56px; display: flex; align-items: center; padding: 0 20px; font-size: 17px; font-weight: 800; color: #262A31; }
.aside :deep(.el-menu) { border-right: 0; }
.header { height: 56px; display: flex; align-items: center; background: #fff; border-bottom: 1px solid #EEF0F3; font-size: 15px; font-weight: 600; color: #262A31; }
.main { padding: 20px; }
</style>
