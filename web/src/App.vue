<script setup lang="ts">
import { ref, onMounted, watch, computed, provide } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NConfigProvider,
  NMessageProvider,
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NMenu,
  NSelect,
  NIcon,
  NButton,
  NSpace,
  NTag,
  darkTheme,
} from 'naive-ui'
import {
  HomeOutline,
  SettingsOutline,
  BarChartOutline,
  LogOutOutline,
} from '@vicons/ionicons5'
import { tenantApi, ipApi, authApi } from './api'
import { useLive } from './composables/useLive'
import type { MenuOption } from 'naive-ui'
import { h } from 'vue'

const isDark = ref<boolean>(true)
const theme = computed(() => darkTheme)

// Premium Color System Overrides (锁定深色模式配色)
const themeOverrides = computed(() => {
  return {
    common: {
      primaryColor: '#60a5fa',
      primaryColorHover: '#93c5fd',
      primaryColorPressed: '#3b82f6',
      successColor: '#10b981',
      warningColor: '#f59e0b',
      errorColor: '#ef4444',
      borderRadius: '8px',
    },
    Card: {
      borderRadius: '12px',
    },
  }
})

const logout = async () => {
  try {
    await authApi.logout()
  } catch {
    // 忽略登出接口失败，前端仍然跳转
  }
  localStorage.removeItem('web_session_id')
  router.push('/login')
}

const updateBodyClass = () => {
  document.documentElement.classList.add('dark')
}

const route = useRoute()
const router = useRouter()

const isLoginPage = computed(() => {
  if (route.name === 'Login') return true
  if (route.name === undefined && window.location.pathname.includes('/login')) return true
  return false
})

const tenants = ref<string[]>([])
// 当前生效租户：只有成功调用 /tenants/switch 写入服务端 Session 后才会更新，
// localStorage 中的值仅作为下次加载时的预选提示，不再直接作为租户凭证使用。
const currentTenant = ref<string>('')
const selectedTenant = ref<string>(localStorage.getItem('currentTenant') || '')
const ipCount = ref(0)

const live = useLive(currentTenant, isLoginPage)
provide('live', live)
provide('currentTenant', currentTenant)

const menuValue = ref(route.name as string)

const menuOptions: MenuOption[] = [
  {
    label: '首页看板',
    key: 'Home',
    icon: () => h(NIcon, null, { default: () => h(HomeOutline) }),
  },
  {
    label: '地址监控管理',
    key: 'Manage',
    icon: () => h(NIcon, null, { default: () => h(SettingsOutline) }),
  },
  {
    label: '历史查询',
    key: 'Analysis',
    icon: () => h(NIcon, null, { default: () => h(BarChartOutline) }),
  },
]

const handleMenuUpdate = (key: string) => {
  router.push({ name: key })
}

watch(
  () => route.name,
  (name) => {
    menuValue.value = name as string
  }
)

const loadTenants = async () => {
  console.log('[app] loadTenants start')
  try {
    const res = await tenantApi.list()
    console.log('[app] tenants response:', res)
    tenants.value = res.data || []
    // 会话内已有生效租户时无需重复切换（例如路由跳转触发的重复加载）
    if (currentTenant.value) {
      return
    }
    // 优先沿用 localStorage 中的预选提示，否则默认选第一个租户；
    // 必须调用 /tenants/switch 写入服务端 Session 后租户才真正生效
    const hinted = localStorage.getItem('currentTenant') || ''
    const target = hinted || tenants.value[0] || ''
    if (target) {
      await switchTenant(target)
    }
  } catch (err) {
    console.error('[app] loadTenants error:', err)
  }
}

const loadIpCount = async () => {
  try {
    const res = await ipApi.list({ current: 1, size: 1 })
    ipCount.value = res.data?.total || 0
  } catch {}
}

// 切换租户：写回服务端 Session，成功后更新本地状态触发各页面刷新
const switchTenant = async (tenantId: string) => {
  try {
    await tenantApi.switch(tenantId)
    currentTenant.value = tenantId
    selectedTenant.value = tenantId
    localStorage.setItem('currentTenant', tenantId)
    console.log('[app] switch tenant ok:', tenantId)
  } catch (err) {
    console.error('[app] switch tenant error:', err)
    // 切换失败时回退下拉框到当前生效租户，避免 UI 与服务端状态不一致
    selectedTenant.value = currentTenant.value
  }
}

const applyTenant = () => {
  if (!selectedTenant.value || selectedTenant.value === currentTenant.value) {
    return
  }
  switchTenant(selectedTenant.value)
}

// 监听租户改变重新拉取 IP 数
watch(currentTenant, () => {
  loadIpCount()
})

// 仅在非登录页状态下且路由已经解析完毕时才加载数据，防范未登录请求接口产生 401 从而造成重定向死循环
watch(
  [isLoginPage, () => route.name],
  ([isLogin, name]) => {
    if (isLogin || name === undefined) {
      return
    }
    loadTenants()
    loadIpCount()
  },
  { immediate: true }
)

onMounted(() => {
  updateBodyClass()
})
</script>

<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <router-view v-if="isLoginPage" />
      <n-layout v-else has-sider style="height: 100vh;" :class="['app-layout', isDark ? 'dark' : '']">
        <n-layout-sider
          bordered
          collapse-mode="width"
          :collapsed-width="64"
          :width="220"
          :native-scrollbar="false"
          class="sidebar-sider"
        >
          <div class="logo-container">
            <span class="logo-text">BS Net Monitor</span>
          </div>
          <n-menu
            :value="menuValue"
            :collapsed-width="64"
            :collapsed-icon-size="22"
            :options="menuOptions"
            @update:value="handleMenuUpdate"
            class="sidebar-menu"
          />
        </n-layout-sider>

        <n-layout class="right-layout">
          <n-layout-header bordered class="app-header">
            <div class="header-left">
              <span class="page-current-title">
                {{ menuOptions.find((m) => m.key === menuValue)?.label }}
              </span>
              <n-tag v-if="menuValue === 'Manage'" size="small" type="info" round class="tag-glow">
                设备总数: {{ ipCount }}
              </n-tag>
              
              <!-- WebSocket Live status -->
              <div class="live-status-group">
                <span 
                  class="live-status-indicator" 
                  :class="{ connected: live.connected.value }"
                ></span>
                <span class="live-status-text">
                  {{ live.connected.value ? '实时已连接' : '实时连接中...' }}
                </span>
              </div>

              <div class="statistics-bar" v-if="live.connected.value">
                <n-tag type="success" size="small" round>在线 {{ live.statistic.value.online }}</n-tag>
                <n-tag type="warning" size="small" round>不稳定 {{ live.statistic.value.unstable }}</n-tag>
                <n-tag type="error" size="small" round>离线 {{ live.statistic.value.offline }}</n-tag>
              </div>
            </div>
            
            <div class="header-right">
              <n-space align="center" size="medium">
                <n-select
                  v-model:value="selectedTenant"
                  :options="tenants.map((t) => ({ label: `租户: ${t}`, value: t }))"
                  placeholder="选择租户"
                  style="width: 180px"
                  filterable
                  tag
                  clearable
                  @update:value="applyTenant"
                />
                


                <n-button circle secondary @click="logout" title="退出登录">
                  <template #icon>
                    <n-icon>
                      <LogOutOutline />
                    </n-icon>
                  </template>
                </n-button>
              </n-space>
            </div>
          </n-layout-header>

          <n-layout-content style="padding: 20px;" class="main-content-layout">
            <div class="content-wrapper">
              <router-view />
            </div>
          </n-layout-content>
        </n-layout>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>

<style scoped>
/* App Layout Custom styling */
.app-layout {
  background: #f3f4f6;
  transition: background-color 0.3s ease;
}
.app-layout.dark {
  background: #0b0b0e;
}

/* Sidebar Styling */
.sidebar-sider {
  background: #ffffff !important;
  box-shadow: 2px 0 8px rgba(0, 0, 0, 0.02) !important;
  transition: all 0.3s ease !important;
}
.app-layout.dark .sidebar-sider {
  background: #121216 !important;
  box-shadow: 2px 0 8px rgba(0, 0, 0, 0.2) !important;
}

.logo-container {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid #f3f4f6;
  padding: 0 16px;
}
.app-layout.dark .logo-container {
  border-bottom-color: #1e1e24;
}

.logo-text {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0.5px;
  background: linear-gradient(135deg, #1d4ed8 0%, #3b82f6 50%, #8b5cf6 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.sidebar-menu {
  padding: 12px 6px;
}

/* Header Styling */
.app-header {
  height: 64px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(255, 255, 255, 0.7) !important;
  backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(243, 244, 246, 0.6) !important;
  transition: all 0.3s ease !important;
}
.app-layout.dark .app-header {
  background: rgba(18, 18, 22, 0.7) !important;
  border-bottom-color: rgba(30, 30, 36, 0.6) !important;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-current-title {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.2px;
}

.tag-glow {
  box-shadow: 0 0 8px rgba(37, 99, 235, 0.15);
}

/* Live indicators */
.live-status-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.live-status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #ef4444;
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.6);
  animation: pulse-offline 2s infinite;
}

.live-status-indicator.connected {
  background-color: #10b981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.6);
  animation: pulse-connected 2s infinite;
}

.live-status-text {
  font-size: 12px;
  font-weight: 600;
  opacity: 0.65;
}

.statistics-bar {
  display: flex;
  gap: 6px;
}

.header-right {
  display: flex;
  align-items: center;
}

.theme-toggle-btn {
  transition: transform 0.3s ease;
}
.theme-toggle-btn:hover {
  transform: rotate(15deg);
}

/* Main Content Panel */
.main-content-layout {
  background: transparent !important;
}

.content-wrapper {
  background: transparent;
  min-height: calc(100vh - 104px);
}

/* Global table & list animation */
:deep(.n-menu-item-content__icon) {
  margin-right: 8px;
  transition: transform 0.2s ease;
}
:deep(.n-menu-item-content:hover .n-menu-item-content__icon) {
  transform: scale(1.1);
}

@keyframes pulse-connected {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.5); }
  70% { transform: scale(1); box-shadow: 0 0 0 4px rgba(16, 185, 129, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

@keyframes pulse-offline {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(239, 68, 68, 0.5); }
  70% { transform: scale(1); box-shadow: 0 0 0 4px rgba(239, 68, 68, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(239, 68, 68, 0); }
}
</style>
