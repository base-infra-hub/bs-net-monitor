<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, inject, computed, type Ref } from 'vue'
import {
  NCard,
  NStatistic,
  NSpace,
  NGrid,
  NGridItem,
  NSelect,
  NButton,
  NForm,
  NFormItem,
  NInput,
  NEmpty,
  NBadge,
  NTooltip,
} from 'naive-ui'
import * as echarts from 'echarts'
import { ipApi, tacticsApi } from '../api'
import type { LiveStatistic, IP, Tactics, PingStatusVO, SubscribeFilterOptions } from '../api'

const ipCount = ref(0)
const tacticsCount = ref(0)
const enabledCount = ref(0)
const tacticsOptions = ref<{ label: string; value: number }[]>([])
const ipOptions = ref<{ label: string; value: number }[]>([])
const allIps = ref<IP[]>([])
const searchQuery = ref('')

const live = inject<{
  connected: { value: boolean }
  statistic: { value: LiveStatistic }
  latestStatuses: { value: Map<number, PingStatusVO> }
  subscribeFilter: (opts: SubscribeFilterOptions) => void
}>('live')

const currentTenant = inject<Ref<string>>('currentTenant')

const filterMode = ref<'all' | 'tactics' | 'ips'>('all')
const selectedTacticsId = ref<number | null>(null)
const selectedIpIds = ref<number[]>([])

const totalChartRef = ref<HTMLDivElement | null>(null)
let totalChart: echarts.ECharts | null = null

const totalHistory = ref<{ time: string; online: number; offline: number; unstable: number }[]>([])
const maxPoints = 60

// 5点延迟历史 (0=离线, 1=不稳定, 2=在线)
const ipHistory = ref<Map<number, number[]>>(new Map())

const modeOptions = [
  { label: '全部 IP', value: 'all' },
  { label: '按策略', value: 'tactics' },
  { label: '按 IP', value: 'ips' },
]

// 过滤当前展示的 IP 列表 (跟随 WebSocket 的实时推送列表，即“推送什么就渲染什么”)
const filteredIps = computed(() => {
  let list = allIps.value

  // 过滤规则：只显示当前 WebSocket 推送了实时状态数据的 IP
  list = list.filter((ip) => live?.latestStatuses.value.has(ip.ipId))

  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase().trim()
    list = list.filter(
      (ip) => ip.name.toLowerCase().includes(query) || ip.ip.includes(query)
    )
  }

  return list
})

// 计算过滤后列表的实时状态统计
const filteredStats = computed(() => {
  let online = 0
  let unstable = 0
  let offline = 0
  let total = filteredIps.value.length

  filteredIps.value.forEach((ip) => {
    const status = live?.latestStatuses.value.get(ip.ipId)
    if (!status) {
      offline++ // 默认未收到包为离线
    } else if (status.status === 2) {
      online++
    } else if (status.status === 1) {
      unstable++
    } else {
      offline++
    }
  })

  return { total, online, unstable, offline }
})

const initCharts = () => {
  if (!totalChartRef.value) return
  
  // 销毁旧图表防止内存泄露
  if (totalChart) {
    totalChart.dispose()
  }

  totalChart = echarts.init(totalChartRef.value)

  const isDark = document.documentElement.classList.contains('dark')
  const axisColor = isDark ? '#888' : '#aaa'
  const splitLineColor = isDark ? '#2c2c32' : '#f0f0f0'

  const option = {
    title: { 
      text: '租户总统计趋势', 
      left: 'center',
      textStyle: {
        fontSize: 15,
        fontWeight: 'bold',
        color: isDark ? '#e3e3e3' : '#1f2329'
      }
    },
    tooltip: { 
      trigger: 'axis',
      axisPointer: { type: 'line' },
      backgroundColor: isDark ? '#24242c' : '#ffffff',
      borderColor: isDark ? '#3b3b44' : '#e0e0e0',
      textStyle: {
        color: isDark ? '#e3e3e3' : '#1f2329'
      }
    },
    legend: { 
      data: ['在线', '离线', '不稳定'], 
      bottom: 0,
      textStyle: {
        color: isDark ? '#ccc' : '#555'
      }
    },
    grid: { left: 45, right: 15, top: 40, bottom: 50 },
    xAxis: { 
      type: 'category', 
      boundaryGap: false, 
      data: [],
      axisLabel: { color: axisColor },
      axisLine: { lineStyle: { color: splitLineColor } }
    },
    yAxis: { 
      type: 'value', 
      minInterval: 1,
      axisLabel: { color: axisColor },
      splitLine: { lineStyle: { color: splitLineColor } }
    },
    series: [
      { 
        name: '在线', 
        type: 'line', 
        smooth: true, 
        data: [], 
        itemStyle: { color: '#18a058' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(24, 160, 88, 0.3)' },
            { offset: 1, color: 'rgba(24, 160, 88, 0)' }
          ])
        }
      },
      { 
        name: '离线', 
        type: 'line', 
        smooth: true, 
        data: [], 
        itemStyle: { color: '#d03050' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(208, 48, 80, 0.3)' },
            { offset: 1, color: 'rgba(208, 48, 80, 0)' }
          ])
        }
      },
      { 
        name: '不稳定', 
        type: 'line', 
        smooth: true, 
        data: [], 
        itemStyle: { color: '#f0a020' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(240, 160, 32, 0.3)' },
            { offset: 1, color: 'rgba(240, 160, 32, 0)' }
          ])
        }
      },
    ],
  }

  totalChart.setOption(option)
}

const updateTotalChart = () => {
  if (!totalChart) return
  totalChart.setOption({
    xAxis: { data: totalHistory.value.map((h) => h.time) },
    series: [
      { data: totalHistory.value.map((h) => h.online) },
      { data: totalHistory.value.map((h) => h.offline) },
      { data: totalHistory.value.map((h) => h.unstable) },
    ],
  })
}

const pushHistory = (
  history: { value: { time: string; online: number; offline: number; unstable: number }[] },
  statistic: LiveStatistic
) => {
  const now = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  history.value.push({ time: now, ...statistic })
  if (history.value.length > maxPoints) {
    history.value.shift()
  }
}

// 监听总体统计变化，更新折线图
watch(
  () => live?.statistic.value,
  (statistic) => {
    if (!statistic) return
    pushHistory(totalHistory, statistic)
    updateTotalChart()
  },
  { deep: true }
)

// 监听最新 IP 状态推送，记录每个 IP 的最后5点状态历史
watch(
  () => live?.latestStatuses.value,
  (statuses) => {
    if (!statuses) return
    statuses.forEach((s: PingStatusVO) => {
      let history = ipHistory.value.get(s.ipId) || []
      history.push(s.status)
      if (history.length > 5) {
        history.shift()
      }
      ipHistory.value.set(s.ipId, history)
    })
  },
  { deep: true }
)

// 响应租户变化
watch(
  () => currentTenant?.value,
  async () => {
    // 切换租户时，重置当前已选过滤条件，以防由于旧策略/旧IP跨租户而报错
    filterMode.value = 'all'
    selectedTacticsId.value = null
    selectedIpIds.value = []

    totalHistory.value = []
    ipHistory.value.clear()
    ipCount.value = 0
    enabledCount.value = 0
    tacticsCount.value = 0
    await loadOptions()
    applyFilter()
    setTimeout(() => {
      initCharts()
    }, 100)
  }
)

const applyFilter = () => {
  const opts: SubscribeFilterOptions = { all: true }
  if (filterMode.value === 'tactics' && selectedTacticsId.value) {
    opts.tacticsId = selectedTacticsId.value
    delete opts.all
  } else if (filterMode.value === 'ips' && selectedIpIds.value.length) {
    opts.ipIds = selectedIpIds.value
    delete opts.all
  }
  live?.subscribeFilter(opts)
}

const loadOptions = async () => {
  const [tacticsRes, ipRes] = await Promise.all([
    tacticsApi.list(),
    ipApi.list({ current: 1, size: 10000 }),
  ])
  const tactics = (tacticsRes.data || []) as Tactics[]
  tacticsOptions.value = tactics.map((t) => ({ label: t.name, value: t.tacticsId }))
  
  const ips = (ipRes.data?.records || []) as IP[]
  allIps.value = ips
  ipOptions.value = ips.map((ip) => ({ label: `${ip.name} (${ip.ip})`, value: ip.ipId }))
  
  // 初始化每个 IP 的历史数据
  ips.forEach((ip) => {
    if (!ipHistory.value.has(ip.ipId)) {
      ipHistory.value.set(ip.ipId, [])
    }
  })
}

const loadStats = async () => {
  try {
    const [ipRes, tacticsRes] = await Promise.all([
      ipApi.list({ current: 1, size: 1 }),
      tacticsApi.list(),
    ])
    ipCount.value = ipRes.data?.total || 0
    enabledCount.value = (await ipApi.list({ current: 1, size: 1, enabled: true })).data?.total || 0
    tacticsCount.value = (tacticsRes.data || []).length
  } catch {}
}

const handleResize = () => {
  totalChart?.resize()
}

onMounted(async () => {
  await loadStats()
  await loadOptions()
  initCharts()
  applyFilter()

  window.addEventListener('resize', handleResize)
  
  // 观察暗黑模式 class 改变来更新图表主题颜色
  const observer = new MutationObserver(() => {
    initCharts()
    updateTotalChart()
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  onUnmounted(() => {
    observer.disconnect()
  })
})

onUnmounted(() => {
  totalChart?.dispose()
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="home-container">
    <h2 class="page-title">系统总览</h2>

    <!-- Premium Stat Cards -->
    <n-grid cols="1 s:3 m:3" x-gap="16" y-gap="16" responsive="screen" class="stat-grid">
      <n-grid-item>
        <div class="glass-card stat-card blue">
          <div class="card-glow"></div>
          <n-statistic label="IP 总数" :value="ipCount">
            <template #suffix>
              <span class="unit">个</span>
            </template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card purple">
          <div class="card-glow"></div>
          <n-statistic label="策略组数" :value="tacticsCount">
            <template #suffix>
              <span class="unit">组</span>
            </template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card green">
          <div class="card-glow"></div>
          <n-statistic label="已启用 IP" :value="enabledCount">
            <template #suffix>
              <span class="unit">个</span>
            </template>
          </n-statistic>
        </div>
      </n-grid-item>
    </n-grid>

    <!-- Sleek Filter Form -->
    <div class="glass-card filter-container" style="margin-top: 20px;">
      <n-form inline :show-feedback="false" class="filter-form">
        <n-form-item label="监控过滤">
          <n-select v-model:value="filterMode" :options="modeOptions" style="width: 140px" />
        </n-form-item>
        <n-form-item v-if="filterMode === 'tactics'" label="选择策略">
          <n-select
            v-model:value="selectedTacticsId"
            :options="tacticsOptions"
            placeholder="选择策略组"
            style="width: 220px"
            clearable
          />
        </n-form-item>
        <n-form-item v-if="filterMode === 'ips'" label="选择 IP">
          <n-select
            v-model:value="selectedIpIds"
            :options="ipOptions"
            placeholder="选择要监控的 IP"
            style="width: 320px"
            multiple
            clearable
            collapse-tags
            :max-tag-count="2"
          />
        </n-form-item>
        <n-form-item>
          <n-button type="primary" class="glow-button" @click="applyFilter">应用过滤</n-button>
        </n-form-item>
      </n-form>
    </div>

    <!-- Main Visual Panels -->
    <n-grid cols="1 l:2" x-gap="20" y-gap="20" responsive="screen" style="margin-top: 20px;">
      <!-- Left Panel: Line Chart -->
      <n-grid-item>
        <div class="glass-card visual-panel">
          <div ref="totalChartRef" style="width: 100%; height: 380px;"></div>
        </div>
      </n-grid-item>

      <!-- Right Panel: Realtime IP Status Grid -->
      <n-grid-item>
        <div class="glass-card visual-panel live-panel">
          <div class="panel-header">
            <div class="header-left">
              <span class="panel-title">实时监控状态看板</span>
              <span class="badge-stats" v-if="filteredIps.length > 0">
                共 {{ filteredStats.total }} · 
                <span class="stat-green">在线 {{ filteredStats.online }}</span> · 
                <span class="stat-yellow">不稳定 {{ filteredStats.unstable }}</span> · 
                <span class="stat-red">离线 {{ filteredStats.offline }}</span>
              </span>
            </div>
            <div class="header-right">
              <n-input
                v-model:value="searchQuery"
                placeholder="搜索名称 / IP"
                size="small"
                clearable
                style="width: 180px"
              />
            </div>
          </div>

          <!-- IP Status Cards Grid -->
          <div class="ip-grid-scroll" :class="{ empty: filteredIps.length === 0 }">
            <n-grid cols="1 s:2" x-gap="12" y-gap="12" responsive="screen" v-if="filteredIps.length > 0">
              <n-grid-item v-for="ip in filteredIps" :key="ip.ipId">
                <div 
                  class="ip-status-card"
                  :class="{
                    online: live?.latestStatuses.value.get(ip.ipId)?.status === 2,
                    unstable: live?.latestStatuses.value.get(ip.ipId)?.status === 1,
                    offline: !live?.latestStatuses.value.get(ip.ipId) || live?.latestStatuses.value.get(ip.ipId)?.status === 0
                  }"
                >
                  <div class="card-header">
                    <span class="ip-name" :title="ip.name">{{ ip.name }}</span>
                    <span class="latency-text">
                      {{ live?.latestStatuses.value.get(ip.ipId)?.latencyMs !== undefined && live?.latestStatuses.value.get(ip.ipId)?.latencyMs !== null ? `${live?.latestStatuses.value.get(ip.ipId)?.latencyMs} ms` : '--' }}
                    </span>
                  </div>
                  
                  <div class="card-body">
                    <span class="ip-address">{{ ip.ip }}</span>
                    <div class="status-indicator">
                      <span class="pulse-dot"></span>
                      <span class="status-label">
                        {{ 
                          !live?.latestStatuses.value.get(ip.ipId) ? '等待数据' :
                          live.latestStatuses.value.get(ip.ipId)?.status === 2 ? '在线' :
                          live.latestStatuses.value.get(ip.ipId)?.status === 1 ? '不稳定' : '离线'
                        }}
                      </span>
                    </div>
                  </div>

                  <!-- 5点历史 timeline -->
                  <div class="card-footer">
                    <span class="footer-label">近期状态:</span>
                    <div class="history-timeline">
                      <n-tooltip trigger="hover" v-for="(val, idx) in 5" :key="idx">
                        <template #trigger>
                          <span 
                            class="timeline-dot"
                            :class="{
                              empty: (ipHistory.get(ip.ipId)?.length || 0) <= idx,
                              online: ipHistory.get(ip.ipId)?.[idx] === 2,
                              unstable: ipHistory.get(ip.ipId)?.[idx] === 1,
                              offline: ipHistory.get(ip.ipId)?.[idx] === 0
                            }"
                          ></span>
                        </template>
                        <span>
                          {{ 
                            (ipHistory.get(ip.ipId)?.length || 0) <= idx ? '暂无数据' :
                            ipHistory.get(ip.ipId)?.[idx] === 2 ? '在线' :
                            ipHistory.get(ip.ipId)?.[idx] === 1 ? '不稳定' : '离线'
                          }}
                        </span>
                      </n-tooltip>
                    </div>
                  </div>
                </div>
              </n-grid-item>
            </n-grid>
            <n-empty v-else description="没有匹配的 IP 设备" style="padding-top: 60px;" />
          </div>
        </div>
      </n-grid-item>
    </n-grid>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 22px;
  font-weight: 700;
  margin-bottom: 20px;
  background: linear-gradient(135deg, #1f2329 0%, #4b5563 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}
.dark .page-title {
  background: linear-gradient(135deg, #f3f4f6 0%, #9ca3af 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Glassmorphism System Cards */
.glass-card {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 12px;
  box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.04);
  padding: 20px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.dark .glass-card {
  background: rgba(24, 24, 28, 0.75);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.2);
}

.stat-card {
  position: relative;
  overflow: hidden;
  border-left: 4px solid #18a058;
}
.stat-card.blue { border-left-color: #3b82f6; }
.stat-card.purple { border-left-color: #8b5cf6; }
.stat-card.green { border-left-color: #10b981; }

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 40px 0 rgba(31, 38, 135, 0.08);
}
.dark .stat-card:hover {
  box-shadow: 0 12px 40px 0 rgba(0, 0, 0, 0.3);
}

.card-glow {
  position: absolute;
  top: -50%;
  right: -50%;
  width: 100%;
  height: 100%;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.15) 0%, rgba(255, 255, 255, 0) 70%);
  transform: rotate(45deg);
  pointer-events: none;
}
.dark .card-glow {
  background: radial-gradient(circle, rgba(255, 255, 255, 0.03) 0%, rgba(255, 255, 255, 0) 70%);
}

.unit {
  font-size: 13px;
  opacity: 0.65;
  margin-left: 4px;
}

/* Filter Styling */
.filter-container {
  padding: 16px 20px;
}
.filter-form :deep(.n-form-item-label) {
  font-weight: 600;
}
.glow-button {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  border: none;
  transition: all 0.3s ease;
}
.glow-button:hover {
  box-shadow: 0 0 12px rgba(37, 99, 235, 0.4);
}

/* Visual Panels */
.visual-panel {
  min-height: 420px;
}

.live-panel {
  display: flex;
  flex-direction: column;
  height: 420px;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  padding-bottom: 12px;
}
.dark class.panel-header {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.panel-title {
  font-size: 15px;
  font-weight: 700;
  margin-right: 12px;
}

.badge-stats {
  font-size: 12px;
  opacity: 0.7;
}
.stat-green { color: #18a058; font-weight: 600; }
.stat-yellow { color: #f0a020; font-weight: 600; }
.stat-red { color: #d03050; font-weight: 600; }

.ip-grid-scroll {
  flex-grow: 1;
  overflow-y: auto;
  padding-right: 4px;
}
.ip-grid-scroll.empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Scrollbar styles */
.ip-grid-scroll::-webkit-scrollbar {
  width: 5px;
}
.ip-grid-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.ip-grid-scroll::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}
.dark .ip-grid-scroll::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
}

/* IP Card grid styling */
.ip-status-card {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-radius: 8px;
  padding: 12px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}
.dark .ip-status-card {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.04);
}

.ip-status-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}
.dark .ip-status-card:hover {
  background: rgba(255, 255, 255, 0.04);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* Pulse color styles */
.ip-status-card.online {
  border-left: 3px solid #18a058;
}
.ip-status-card.unstable {
  border-left: 3px solid #f0a020;
}
.ip-status-card.offline {
  border-left: 3px solid #d03050;
}

.ip-status-card.online:hover {
  box-shadow: 0 4px 16px rgba(24, 160, 88, 0.15);
}
.ip-status-card.unstable:hover {
  box-shadow: 0 4px 16px rgba(240, 160, 32, 0.15);
}
.ip-status-card.offline:hover {
  box-shadow: 0 4px 16px rgba(208, 48, 80, 0.15);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.ip-name {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 140px;
}

.latency-text {
  font-size: 13px;
  font-weight: 700;
  font-family: monospace;
}
.online .latency-text { color: #18a058; }
.unstable .latency-text { color: #f0a020; }
.offline .latency-text { color: #d03050; }

.card-body {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.ip-address {
  font-size: 12px;
  opacity: 0.65;
  font-family: monospace;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
  background-color: #d03050;
}
.online .pulse-dot {
  background-color: #18a058;
  animation: pulse-green 2s infinite;
}
.unstable .pulse-dot {
  background-color: #f0a020;
  animation: pulse-yellow 2s infinite;
}
.offline .pulse-dot {
  background-color: #d03050;
  animation: pulse-red 2s infinite;
}

.status-label {
  font-size: 11px;
  font-weight: 600;
}
.online .status-label { color: #18a058; }
.unstable .status-label { color: #f0a020; }
.offline .status-label { color: #d03050; }

/* Timeline history indicator */
.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px dashed rgba(0, 0, 0, 0.05);
  padding-top: 8px;
  margin-top: 4px;
}
.dark .card-footer {
  border-top-color: rgba(255, 255, 255, 0.05);
}

.footer-label {
  font-size: 11px;
  opacity: 0.55;
}

.history-timeline {
  display: flex;
  gap: 4px;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  display: inline-block;
}
.timeline-dot.empty {
  background-color: #e0e0e0;
}
.dark .timeline-dot.empty {
  background-color: #2c2c32;
}
.timeline-dot.online {
  background-color: rgba(24, 160, 88, 0.85);
}
.timeline-dot.unstable {
  background-color: rgba(240, 160, 32, 0.85);
}
.timeline-dot.offline {
  background-color: rgba(208, 48, 80, 0.85);
}

/* Animations */
@keyframes pulse-green {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(24, 160, 88, 0.5); }
  70% { transform: scale(1); box-shadow: 0 0 0 5px rgba(24, 160, 88, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(24, 160, 88, 0); }
}
@keyframes pulse-yellow {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(240, 160, 32, 0.5); }
  70% { transform: scale(1); box-shadow: 0 0 0 5px rgba(240, 160, 32, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(240, 160, 32, 0); }
}
@keyframes pulse-red {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(208, 48, 80, 0.5); }
  70% { transform: scale(1); box-shadow: 0 0 0 5px rgba(208, 48, 80, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(208, 48, 80, 0); }
}
</style>
