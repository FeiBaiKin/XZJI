<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { logout, me } from '../auth'
import ChangePasswordDialog from '../components/ChangePasswordDialog.vue'

const route = useRoute()
const router = useRouter()
const pwOpen = ref(false)

async function onCommand(cmd: string) {
  if (cmd === 'password') pwOpen.value = true
  if (cmd === 'logout') {
    await logout()
    await router.replace({ name: 'login' })
  }
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="208px" class="aside">
      <div class="logo">XZJI 后台</div>
      <el-menu :default-active="route.path" router>
        <el-menu-item index="/">仪表盘</el-menu-item>
        <el-menu-item index="/users">用户管理</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span>{{ route.meta.title }}</span>
        <el-dropdown trigger="click" @command="onCommand">
          <span class="me">{{ me?.nickname }}<small>{{ me?.email }}</small></span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main">
        <RouterView />
      </el-main>
    </el-container>
    <ChangePasswordDialog v-model="pwOpen" />
  </el-container>
</template>

<style scoped>
.layout { height: 100%; }
.aside { background: #fff; border-right: 1px solid #EEF0F3; }
.logo { height: 56px; display: flex; align-items: center; padding: 0 20px; font-size: 17px; font-weight: 800; color: #262A31; }
.aside :deep(.el-menu) { border-right: 0; }
.header { height: 56px; display: flex; align-items: center; justify-content: space-between; background: #fff; border-bottom: 1px solid #EEF0F3; font-size: 15px; font-weight: 600; color: #262A31; }
.me { display: flex; flex-direction: column; align-items: flex-end; cursor: pointer; font-size: 14px; line-height: 1.3; color: #262A31; }
.me small { font-size: 12px; font-weight: 400; color: #8A9099; }
.main { padding: 20px; }
</style>
