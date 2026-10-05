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
      <el-menu :default-active="(route.meta.menu as string) ?? route.path" router>
        <el-menu-item index="/">仪表盘</el-menu-item>
        <el-menu-item-group title="内容">
          <el-menu-item index="/posts">内容列表</el-menu-item>
          <el-menu-item index="/categories">分类管理</el-menu-item>
          <el-menu-item index="/tags">标签管理</el-menu-item>
          <el-menu-item index="/comments">评论管理</el-menu-item>
          <el-menu-item index="/link-reports">失效反馈</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="运营">
          <el-menu-item index="/settings/home">首页设置</el-menu-item>
          <el-menu-item index="/settings/site">站点设置</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="用户">
          <el-menu-item index="/users">用户管理</el-menu-item>
          <el-menu-item index="/levels">会员与套餐</el-menu-item>
        </el-menu-item-group>
        <el-menu-item-group title="交易">
          <el-menu-item index="/orders">订单管理</el-menu-item>
          <el-menu-item index="/ledger">余额流水</el-menu-item>
          <el-menu-item index="/settings/payment">支付设置</el-menu-item>
        </el-menu-item-group>
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
