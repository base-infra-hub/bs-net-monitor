<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, inject, watch, nextTick, type Ref } from 'vue'
import {
  NGrid,
  NGridItem,
  NStatistic,
  NSpace,
  NSelect,
  NDatePicker,
  NSpin,
  NModal,
  NDescriptions,
  NDescriptionsItem,
  NTag,
  NTabs,
  NTabPane,
} from 'naive-ui'
import * as echarts from 'echarts'
import { fmtDateTime, fmtDate, fmtTime, fmtHM, fmtMD, fmtMDHM } from '../utils/datetime'
import {
  historyApi,
  ipApi,
  tacticsApi,
  type IP,
  type Tactics,
  type HistorySummary,
  type HistoryWindow,
  type HistoryRecord,
} from '../api'

const currentTenant = inject<Ref<string>>('currentTenant')

// 查询条件：日期范围默认今天一天（最大跨度 7 天），策略/IP 均可选（不传则查全部）
const startOfDay = (ts: number) => {
  const d = new Date(ts)
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}
const dateRange = ref<[number, number]>([startOfDay(Date.now()), startOfDay(Date.now())])
const tacticsId = ref<number | null>(null)
const ipId = ref<number | null>(null)

const loading = ref(false)
const summary = ref<HistorySummary | null>(null)
const windows = ref<HistoryWindow[]>([])

// 状态条按日期 Tab 切换：每天一个 Tab，默认显示第一天
const currentDayIndex = ref(0)
const dayTabs = computed(() => {
  const tabs: string[] = []
  for (let i = 0; i < windows.value.length; i += MINUTES_PER_DAY) {
    tabs.push(fmtMD(windows.value[i]?.time) ?? '')
  }
  return tabs
})

// 当前选中那天的窗口切片
const currentDayWindows = computed(() => {
  const start = currentDayIndex.value * MINUTES_PER_DAY
  return windows.value.slice(start, start + MINUTES_PER_DAY)
})

// 是否多天范围（用于曲线 x 轴标签）
const isMultiDay = computed(() => windows.value.length > MINUTES_PER_DAY)

watch(currentDayIndex, () => {
  // 只需要重绘状态条，趋势图和饼图不受影响
  if (stripChart) {
    const c = themeColors()
    stripChart.setOption(buildStripOption(c))
  }
})

const tacticsList = ref<Tactics[]>([])
const ipList = ref<IP[]>([])

const tacticsOptions = computed(() =>
  tacticsList.value.map((t) => ({ label: t.name, value: t.tacticsId }))
)

// 选好策略和 IP 后才允许查询
const canQuery = computed(() => tacticsId.value !== null && ipId.value !== null)

// IP 下拉跟随策略联动：选了策略后只展示该策略下的 IP
const ipOptions = computed(() =>
  ipList.value
    .filter((i) => tacticsId.value === null || i.tacticsId === tacticsId.value)
    .map((i) => ({ label: `${i.name} (${i.ip})`, value: i.ipId }))
)

// 切换策略时，若已选 IP 不属于新策略则清空
const onTacticsChange = (v: number | null) => {
  tacticsId.value = v
  if (v !== null && ipId.value !== null) {
    const ip = ipList.value.find((i) => i.ipId === ipId.value)
    if (ip && ip.tacticsId !== v) ipId.value = null
  }
}

const stripChartRef = ref<HTMLDivElement | null>(null)
const trendChartRef = ref<HTMLDivElement | null>(null)
const pieChartRef = ref<HTMLDivElement | null>(null)
let stripChart: echarts.ECharts | null = null
let trendChart: echarts.ECharts | null = null
let pieChart: echarts.ECharts | null = null

// 分钟详情弹窗
const showMinuteModal = ref(false)
const activeWindow = ref<HistoryWindow | null>(null)
const detailRecords = ref<HistoryRecord[]>([])
const detailLoading = ref(false)
const minuteChartRef = ref<HTMLDivElement | null>(null)
let minuteChart: echarts.ECharts | null = null

const openMinute = async (index: number) => {
  const w = windows.value[index]
  if (!w) return
  activeWindow.value = w
  showMinuteModal.value = true
  detailLoading.value = true
  try {
    const res = await historyApi.detail({
      time: w.time,
      tacticsId: tacticsId.value ?? undefined,
      ipId: ipId.value ?? undefined,
    })
    detailRecords.value = res.data || []
  } catch {
    detailRecords.value = []
    // 拦截器已统一提示错误
  } finally {
    detailLoading.value = false
  }
  await nextTick()
  renderMinuteChart()
}

// 状态点颜色：稳定绿 / 不稳定黄 / 超时红，与曲线点一致
const pointColor = (status: number) =>
  status === 2 ? '#18a058' : status === 1 ? '#f0a020' : '#d03050'

// 弹窗内分钟明细曲线：每个点是一条探测记录，断联画在 -1（轴下方）
const renderMinuteChart = () => {
  if (!minuteChartRef.value) return
  if (minuteChart) minuteChart.dispose()
  minuteChart = echarts.init(minuteChartRef.value)
  const c = themeColors()
  const minuteBreak = computeLatencyBreak(
    detailRecords.value.map((r) => r.latencyMs).filter((v): v is number => v != null && v > 0)
  )
  minuteChart.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' },
      formatter: (params: any) => {
        const p = params[0]
        if (!p) return ''
        const r = detailRecords.value[p.dataIndex]
        if (!r) return ''
        return [
          `<b>${fmtDateTime(r.time)}</b>`,
          `设备：${r.name}（${r.ip}）`,
          `状态：${statusText(r.status)}`,
          `延迟：${r.latencyMs ?? '-'} ms`,
        ].join('<br/>')
      },
    },
    grid: { left: 55, right: 25, top: 25, bottom: 30 },
    xAxis: {
      type: 'category',
      data: detailRecords.value.map((r) => fmtTime(r.time)),
      axisLabel: { color: c.axisColor },
      axisLine: { lineStyle: { color: c.splitLineColor } },
    },
    yAxis: {
      type: 'value',
      name: '延迟 (ms)',
      min: (v: any) => {
        const max = v.max ?? 0
        return -Math.max(Math.ceil(max * 0.12), 2)
      },
      ...(minuteBreak ? { breaks: [minuteBreak], breakArea: { show: false } } : {}),
      axisLabel: { color: c.axisColor },
      splitLine: { lineStyle: { color: c.splitLineColor } },
    },
    series: [
      {
        name: '延迟',
        type: 'line',
        smooth: false,
        showSymbol: true,
        symbolSize: 7,
        connectNulls: true,
        data: detailRecords.value.map((r) => r.latencyMs ?? -1),
        itemStyle: {
          color: (p: any) => {
            const r = detailRecords.value[p.dataIndex]
            if (!r) return '#3b82f6'
            return pointColor(r.status)
          }
        },
        lineStyle: {
          color: '#3b82f6',
          width: 2,
          shadowBlur: 8,
          shadowColor: 'rgba(59, 130, 246, 0.8)',
          shadowOffsetY: 1
        },
        markLine: {
          silent: true,
          symbol: 'none',
          label: {
            show: true,
            position: 'end',
            formatter: '离线分界',
            color: '#ef4444',
            fontSize: 9
          },
          lineStyle: { type: 'solid', color: 'rgba(239, 68, 68, 0.6)', width: 1.5 },
          data: [{ yAxis: 0 }],
        },
      },
    ],
  })
}

const statusText = (s: number) =>
  ({ 2: '稳定', 1: '不稳定', 0: '断联', '-1': '无数据' })[s] ?? '未知'

const statusTagType = (s?: number) =>
  s === 2 ? 'success' : s === 1 ? 'warning' : s === 0 ? 'error' : 'default'

// 后端按服务器本地时区解释日期，这里同样用浏览器本地时间格式化
const formatDate = (ts: number) => {
  const d = new Date(ts)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

// 禁用未来日期；选定第一个日期后，第二个日期只允许在其前后 6 天内（跨度最大 7 天）
const isDateDisabled = (ts: number, type: any, range: any) => {
  if (ts > Date.now()) return true
  if (type === 'end' && range && range[0] !== null) {
    return Math.abs(ts - range[0]) > 6 * 24 * 3600 * 1000
  }
  return false
}

const loadOptions = async () => {
  try {
    const [ipRes, tacticsRes] = await Promise.all([
      ipApi.list({ current: 1, size: 999 }),
      tacticsApi.list(),
    ])
    ipList.value = ipRes.data?.records || []
    tacticsList.value = tacticsRes.data || []
  } catch {
    // 拦截器已统一提示错误
  }
}

const fetchHistory = async () => {
  // 未选好策略和 IP 时不查询，清空展示
  if (!canQuery.value) {
    summary.value = null
    windows.value = []
    updateCharts()
    return
  }
  loading.value = true
  try {
    const res = await historyApi.query({
      startDate: formatDate(dateRange.value[0]),
      endDate: formatDate(dateRange.value[1]),
      tacticsId: tacticsId.value ?? undefined,
      ipId: ipId.value ?? undefined,
    })
    summary.value = res.data.summary
    windows.value = res.data.windows
    updateCharts()
  } catch {
    // 拦截器已统一提示错误
  } finally {
    loading.value = false
  }
}

const themeColors = () => {
  const isDark = document.documentElement.classList.contains('dark')
  return {
    isDark,
    axisColor: isDark ? '#888' : '#aaa',
    splitLineColor: isDark ? '#2c2c32' : '#f0f0f0',
    titleColor: isDark ? '#e3e3e3' : '#1f2329',
    legendColor: isDark ? '#ccc' : '#555',
    noDataColor: isDark ? '#4b5563' : '#d1d5db',
  }
}

// 计算百分位点，用于轴断裂时区分正常值与异常高值
const percentile = (sorted: number[], p: number) => {
  if (sorted.length === 0) return 0
  const idx = (sorted.length - 1) * p
  const lower = Math.floor(idx)
  const upper = Math.ceil(idx)
  const weight = idx - lower
  return sorted[lower] * (1 - weight) + sorted[upper] * weight
}

// 当存在极端高延迟点导致其它曲线被压平时，给 Y 轴加一个断裂段，保留正常区间的曲线感
const computeLatencyBreak = (values: number[]): { start: number; end: number } | undefined => {
  const valid = values.filter((v) => v != null && v > 0)
  if (valid.length < 10) return undefined
  const sorted = [...valid].sort((a, b) => a - b)
  const p90 = percentile(sorted, 0.9)
  const max = sorted[sorted.length - 1]
  // 只有异常值明显高于主体数据时才断裂，避免正常波动也产生断裂
  if (max > p90 * 2 && max > p90 + 100) {
    return { start: p90, end: max }
  }
  return undefined
}

const MINUTES_PER_DAY = 1440

// 一天 1440 分钟的横轴标签（00:00 - 23:59）
const minuteLabels = Array.from({ length: MINUTES_PER_DAY }, (_, i) => {
  const h = String(Math.floor(i / 60)).padStart(2, '0')
  const m = String(i % 60).padStart(2, '0')
  return `${h}:${m}`
})

// 分钟状态热力图：只渲染当前 Tab 选中那天的 1440 分钟，
// 格子按主导状态着色，悬浮显示该分钟的详细统计
const buildStripOption = (c: ReturnType<typeof themeColors>): echarts.EChartsOption => {
  const dayWindows = currentDayWindows.value
  const dayTitle = fmtDate(dayWindows[0]?.time) ?? ''
  return {
    title: {
      text: dayTitle ? `${dayTitle} 分钟状态` : '分钟状态分布',
      left: 'center',
      textStyle: { fontSize: 13, fontWeight: 'bold', color: c.titleColor },
    },
    tooltip: {
      formatter: (p: any) => {
        const w = dayWindows[p.value[0]]
        if (!w) return ''
        return [
          fmtDateTime(w.time),
          `状态：${statusText(w.status)}`,
          `稳定 ${w.onlineCount} / 不稳定 ${w.unstableCount} / 断联 ${w.offlineCount}（共 ${w.total} 次）`,
          `平均延迟：${w.avgLatencyMs ?? '-'} ms`,
        ].join('<br/>')
      },
    },
    grid: { left: 20, right: 20, top: 34, bottom: 44 },
    xAxis: {
      type: 'category',
      data: minuteLabels,
      axisLabel: {
        color: c.axisColor,
        interval: (index: number) => index % 120 === 0,
        formatter: (_value: string, index: number) => fmtHM(dayWindows[index]?.time ?? ''),
      },
      axisLine: { lineStyle: { color: c.splitLineColor } },
      axisTick: { show: false },
    },
    yAxis: { type: 'category', data: ['状态'], show: false },
    visualMap: {
      type: 'piecewise',
      orient: 'horizontal',
      left: 'center',
      bottom: 0,
      itemWidth: 12,
      itemHeight: 12,
      textStyle: { color: c.legendColor },
      pieces: [
        { value: 2, label: '稳定', color: '#18a058' },
        { value: 1, label: '不稳定', color: '#f0a020' },
        { value: 0, label: '断联', color: '#d03050' },
        { value: -1, label: '无数据', color: c.noDataColor },
      ],
    },
    series: [
      {
        name: '状态',
        type: 'heatmap',
        data: dayWindows.map((w, i) => [i, 0, w.status]),
        emphasis: { itemStyle: { shadowBlur: 4, shadowColor: 'rgba(0,0,0,0.4)' } },
      },
    ],
  }
}

// 分钟延迟曲线：每个点的高度是该分钟的平均延迟，颜色为后端按策略阈值判定的
// 延迟状态（稳定绿 / 不稳定黄）；该分钟全部断联时画在 -1（轴下方，红色）；无数据则断开
const buildTrendOption = (c: ReturnType<typeof themeColors>): echarts.EChartsOption => {
  // 收集所有有效延迟值，用于判断是否需要对极端高延迟做轴断裂
  const latencyValues = windows.value.flatMap((w) =>
    [w.avgLatencyMs, w.minLatencyMs, w.maxLatencyMs].filter((v): v is number => v != null && v > 0)
  )
  const latencyBreak = computeLatencyBreak(latencyValues)

  return {
  title: {
    text: '分钟延迟与稳定性趋势',
    left: 'left',
    textStyle: { fontSize: 14, fontWeight: 'bold', color: c.titleColor },
  },
  legend: {
    data: ['稳定次数', '不稳定次数', '断联次数', '平均延迟', '延迟区间'],
    right: 10,
    top: 5,
    textStyle: { color: c.axisColor, fontSize: 11 }
  },
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'line' },
    formatter: (params: any) => {
      const w = windows.value[params[0]?.dataIndex]
      if (!w) return ''
      const latency =
        w.total === 0 ? '无数据' : w.latencyStatus === 0 ? '全部断联' : `${w.avgLatencyMs} ms`
      const minMax =
        w.minLatencyMs !== null && w.maxLatencyMs !== null
          ? `<span style="color:#94a3b8;font-size:11px">延迟区间：${w.minLatencyMs} ~ ${w.maxLatencyMs} ms</span>`
          : ''
      return [
        `<b>${fmtDateTime(w.time)}</b>`,
        `平均延迟：${latency}`,
        minMax,
        `<span style="color:#18a058">●</span> 稳定探测：${w.onlineCount} 次`,
        `<span style="color:#f0a020">●</span> 不稳定探测：${w.unstableCount} 次`,
        `<span style="color:#d03050">●</span> 断联探测：${w.offlineCount} 次`,
        `总计探测：${w.total} 次`
      ].filter(Boolean).join('<br/>')
    },
  },
  grid: { left: 55, right: 55, top: 55, bottom: 58 },
  xAxis: {
    type: 'category',
    data: windows.value.map((w) => (isMultiDay.value ? fmtMDHM(w.time) : fmtHM(w.time))),
    axisLabel: { color: c.axisColor },
    axisLine: { lineStyle: { color: c.splitLineColor } },
  },
  yAxis: [
    {
      type: 'value',
      name: '延迟 (ms)',
      min: (v: any) => {
        const max = v.max ?? 0
        return -Math.max(Math.ceil(max * 0.12), 2)
      },
      ...(latencyBreak ? { breaks: [latencyBreak], breakArea: { show: false } } : {}),
      axisLabel: { color: c.axisColor },
      splitLine: { lineStyle: { color: c.splitLineColor } },
    },
    {
      type: 'value',
      name: '探测次数',
      min: 0,
      minInterval: 1,
      axisLabel: { color: c.axisColor },
      splitLine: { show: false },
    }
  ],
  dataZoom: [
    { type: 'inside' },
    { type: 'slider', height: 16, bottom: 10, textStyle: { color: c.axisColor } },
  ],
  series: [
    {
      name: '稳定次数',
      type: 'bar',
      stack: 'count',
      yAxisIndex: 1,
      barWidth: '60%',
      itemStyle: { color: 'rgba(24, 160, 88, 0.25)' },
      data: windows.value.map((w) => w.onlineCount)
    },
    {
      name: '不稳定次数',
      type: 'bar',
      stack: 'count',
      yAxisIndex: 1,
      itemStyle: { color: 'rgba(240, 160, 32, 0.35)' },
      data: windows.value.map((w) => w.unstableCount)
    },
    {
      name: '断联次数',
      type: 'bar',
      stack: 'count',
      yAxisIndex: 1,
      itemStyle: { color: 'rgba(208, 48, 80, 0.35)' },
      data: windows.value.map((w) => w.offlineCount)
    },
    {
      name: '平均延迟',
      type: 'line',
      smooth: true,
      showSymbol: true,
      symbolSize: 6,
      connectNulls: false,
      sampling: 'lttb',
      data: windows.value.map((w) => {
        if (w.total === 0) return null
        if (w.latencyStatus === 0) return -1
        return w.avgLatencyMs
      }),
      itemStyle: {
        color: (p: any) => {
          const w = windows.value[p.dataIndex]
          if (!w) return '#3b82f6'
          return pointColor(w.latencyStatus)
        }
      },
      lineStyle: {
        color: '#3b82f6',
        width: 3,
        shadowBlur: 10,
        shadowColor: 'rgba(59, 130, 246, 0.8)',
        shadowOffsetY: 2
      },
      markLine: {
        silent: true,
        symbol: 'none',
        label: {
          show: true,
          position: 'end',
          formatter: '离线分界',
          color: '#ef4444',
          fontSize: 9
        },
        lineStyle: { type: 'solid', color: 'rgba(239, 68, 68, 0.6)', width: 1.5 },
        data: [{ yAxis: 0 }],
      },
    },
    // min-max 延迟区间带：用堆叠面积图实现
    // 第一层：以 minLatencyMs 为下基线（面积透明，只撑高度）
    {
      name: '最小延迟',
      type: 'line',
      smooth: true,
      showSymbol: false,
      connectNulls: false,
      legendHoverLink: false,
      sampling: 'lttb',
      silent: true,
      data: windows.value.map((w) => {
        if (w.total === 0 || w.latencyStatus === 0 || w.minLatencyMs === null) return null
        return w.minLatencyMs
      }),
      lineStyle: { opacity: 0 },
      itemStyle: { opacity: 0 },
      // 面积完全透明，仅用于给下面的带状图提供基线
      areaStyle: { color: 'transparent' },
      stack: 'latency-band',
      tooltip: { show: false },
    },
    // 第二层：以 (maxLatencyMs - minLatencyMs) 为高度，填充半透明蓝色区间带
    {
      name: '最大延迟',
      type: 'line',
      smooth: true,
      showSymbol: false,
      connectNulls: false,
      legendHoverLink: false,
      sampling: 'lttb',
      silent: true,
      data: windows.value.map((w) => {
        if (w.total === 0 || w.latencyStatus === 0 || w.minLatencyMs === null || w.maxLatencyMs === null) return null
        return w.maxLatencyMs - w.minLatencyMs
      }),
      lineStyle: { opacity: 0 },
      itemStyle: { opacity: 0 },
      areaStyle: {
        color: 'rgba(59, 130, 246, 0.15)',
        shadowBlur: 4,
        shadowColor: 'rgba(59, 130, 246, 0.1)',
      },
      stack: 'latency-band',
      tooltip: { show: false },
    },
  ],
  }
}

// 查询日期范围标签，用于饼图标题
const dateRangeLabel = computed(() => {
  const s = formatDate(dateRange.value[0])
  const e = formatDate(dateRange.value[1])
  return s === e ? s : `${s} ~ ${e}`
})

// 状态占比饼图：基于查询范围内的 summary 汇总数据
const buildPieOption = (c: ReturnType<typeof themeColors>): echarts.EChartsOption => {
  const s = summary.value
  const onlineCount = s?.onlineCount ?? 0
  const unstableCount = s?.unstableCount ?? 0
  const offlineCount = s?.offlineCount ?? 0
  const total = onlineCount + unstableCount + offlineCount
  const onlineRate = s?.onlineRate ?? 0
  const avgLatency = s?.avgLatencyMs

  const hasData = total > 0

  const makeSlice = (
    value: number,
    name: string,
    colorTop: string,
    colorBot: string,
    glow: string,
  ) => ({
    value,
    name,
    itemStyle: {
      color: {
        type: 'linear' as const,
        x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: colorTop },
          { offset: 1, color: colorBot },
        ],
      },
      shadowBlur: 16,
      shadowColor: glow,
    },
  })

  const data = hasData
    ? [
        makeSlice(onlineCount,   '稳定',   '#34d882', '#18a058', 'rgba(24,160,88,0.55)'),
        makeSlice(unstableCount, '不稳定', '#ffd54f', '#f0a020', 'rgba(240,160,32,0.55)'),
        makeSlice(offlineCount,  '断联',   '#ff6b6b', '#d03050', 'rgba(208,48,80,0.55)'),
      ].filter((d) => d.value > 0)
    : [{ value: 1, name: '无数据', itemStyle: { color: c.noDataColor } as any }]

  // 圆环中心指标：稳定率 大字 + 副标题 "稳定率"（居中于画布，与饼图圆心对齐）
  const centerGraphic = hasData
    ? {
        elements: [
          {
            type: 'text' as const,
            left: 'center' as const,
            top: 'middle' as const,
            style: {
              text: `{pct|${onlineRate.toFixed(1)}%}\n{label|稳定率}`,
              textAlign: 'center' as const,
              textVerticalAlign: 'middle' as const,
              rich: {
                pct: {
                  fontSize: 28,
                  fontWeight: 'bold' as const,
                  fill: onlineRate >= 80 ? '#18a058' : onlineRate >= 50 ? '#f0a020' : '#d03050',
                  lineHeight: 32,
                },
                label: {
                  fontSize: 12,
                  fill: c.axisColor,
                  lineHeight: 16,
                },
              },
            },
          },
        ],
      }
    : undefined

  return {
    title: [
      {
        // 主标题：查询范围
        text: `状态占比  {range|${dateRangeLabel.value}}`,
        left: 'center',
        top: 6,
        textStyle: {
          fontSize: 13,
          fontWeight: 'bold' as const,
          color: c.titleColor,
          rich: {
            range: {
              fontSize: 11,
              fontWeight: 'normal' as const,
              color: c.axisColor,
            },
          },
        },
      },
      // 副标题行：总探测次数 + 平均延迟
      ...(hasData
        ? [
            {
              text: [
                `总探测 ${total.toLocaleString()} 次`,
                avgLatency != null ? `均延迟 ${avgLatency} ms` : '',
              ]
                .filter(Boolean)
                .join('   '),
              left: 'center',
              top: 34,
              textStyle: {
                fontSize: 10,
                fontWeight: 'normal' as const,
                color: c.axisColor,
              },
            },
          ]
        : []),
    ],
    ...(centerGraphic ? { graphic: centerGraphic } : {}),
    tooltip: {
      trigger: 'item',
      backgroundColor: '#1e1e28',
      borderColor: '#3b3b50',
      borderWidth: 1,
      padding: [8, 12],
      textStyle: { color: '#e3e3e3', fontSize: 12 },
      formatter: (p: any) =>
        p.name === '无数据'
          ? '<span style="color:#888">该时间范围内无探测数据</span>'
          : [
              `<b style="color:${p.color?.colorStops?.[0]?.color ?? p.color}">${p.name}</b>`,
              `探测次数：<b>${p.value.toLocaleString()}</b> 次`,
              `占比：<b>${p.percent}%</b>`,
            ].join('<br/>'),
    },
    legend: {
      orient: 'horizontal',
      bottom: 4,
      textStyle: { color: c.axisColor, fontSize: 11 },
      itemWidth: 9,
      itemHeight: 9,
      itemGap: 14,
      icon: 'circle',
    },
    series: [
      {
        name: '状态占比',
        type: 'pie',
        // 外圈：主数据
        radius: ['40%', '64%'],
        center: ['50%', '50%'],
        data,
        label: {
          show: hasData,
          color: c.legendColor,
          fontSize: 11,
          lineHeight: 16,
          formatter: (p: any) =>
            p.name === '无数据' ? '' : `{name|${p.name}}\n{pct|${p.percent}%}`,
          rich: {
            name: { fontSize: 11, color: c.legendColor },
            pct: { fontSize: 12, fontWeight: 'bold', color: c.titleColor },
          },
        },
        labelLine: {
          length: 8,
          length2: 10,
          smooth: true,
          lineStyle: { width: 1.2, opacity: 0.7 },
        },
        emphasis: {
          scale: true,
          scaleSize: 8,
          itemStyle: {
            shadowBlur: 24,
            shadowOffsetX: 0,
            shadowColor: 'rgba(255,255,255,0.25)',
          },
        },
        itemStyle: {
          borderRadius: 7,
          borderColor: '#14141c',
          borderWidth: 2.5,
        },
        animationType: 'scale',
        animationEasing: 'elasticOut',
        animationDelay: 100,
      },
    ],
  }
}

const updateCharts = () => {
  if (!stripChart || !trendChart || !pieChart) return
  const c = themeColors()
  stripChart.setOption(buildStripOption(c))
  trendChart.setOption(buildTrendOption(c))
  pieChart.setOption(buildPieOption(c))
  // 天数变化会改变状态条面板高度，重排后需要 resize
  nextTick(handleResize)
}

const initCharts = () => {
  if (stripChartRef.value) {
    if (stripChart) stripChart.dispose()
    stripChart = echarts.init(stripChartRef.value)
  }
  if (trendChartRef.value) {
    if (trendChart) trendChart.dispose()
    trendChart = echarts.init(trendChartRef.value)
    trendChart.on('click', (p: any) => {
      void openMinute(p.dataIndex)
    })
  }
  if (pieChartRef.value) {
    if (pieChart) pieChart.dispose()
    pieChart = echarts.init(pieChartRef.value)
  }
  updateCharts()
}

watch([dateRange, tacticsId, ipId], fetchHistory)

// 弹窗关闭时销毁分钟明细图，避免残留实例
watch(showMinuteModal, (show) => {
  if (!show) {
    minuteChart?.dispose()
    minuteChart = null
  }
})

watch(
  () => currentTenant?.value,
  () => {
    // 策略与 IP 是租户隔离的，切换租户时清空过滤并重新加载
    tacticsId.value = null
    ipId.value = null
    loadOptions()
    fetchHistory()
  }
)

const handleResize = () => {
  stripChart?.resize()
  trendChart?.resize()
  pieChart?.resize()
}

onMounted(() => {
  setTimeout(() => {
    initCharts()
    loadOptions()
  }, 100)

  window.addEventListener('resize', handleResize)

  const observer = new MutationObserver(() => {
    initCharts()
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  onUnmounted(() => {
    observer.disconnect()
  })
})

onUnmounted(() => {
  stripChart?.dispose()
  trendChart?.dispose()
  pieChart?.dispose()
  minuteChart?.dispose()
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="analysis-container">
    <!-- 标题行：左侧标题，右侧查询条件 -->
    <div class="title-row">
      <h2 class="page-title">历史数据查询</h2>
      <n-space align="center" :size="10" class="filter-group">
        <n-spin v-if="loading" :size="16" />
        <n-date-picker
          v-model:value="dateRange"
          type="daterange"
          :clearable="false"
          :is-date-disabled="isDateDisabled"
          style="width: 250px"
        />
        <n-select
          :value="tacticsId"
          :options="tacticsOptions"
          clearable
          placeholder="全部策略"
          style="width: 150px"
          @update:value="onTacticsChange"
        />
        <n-select
          v-model:value="ipId"
          :options="ipOptions"
          clearable
          filterable
          placeholder="全部 IP"
          style="width: 200px"
        />
      </n-space>
    </div>

    <!-- 当天汇总指标 -->
    <n-grid cols="2 s:3 m:5" x-gap="12" y-gap="12" responsive="screen" class="metric-grid">
      <n-grid-item>
        <div class="glass-card stat-card green">
          <n-statistic label="稳定率" :value="summary?.onlineRate ?? 0">
            <template #suffix><span class="unit">%</span></template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card blue">
          <n-statistic label="稳定次数" :value="summary?.onlineCount ?? 0">
            <template #suffix><span class="unit">次</span></template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card yellow">
          <n-statistic label="不稳定次数" :value="summary?.unstableCount ?? 0">
            <template #suffix><span class="unit">次</span></template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card red">
          <n-statistic label="断联次数" :value="summary?.offlineCount ?? 0">
            <template #suffix><span class="unit">次</span></template>
          </n-statistic>
        </div>
      </n-grid-item>
      <n-grid-item>
        <div class="glass-card stat-card purple">
          <n-statistic label="平均延迟" :value="summary?.avgLatencyMs ?? '-'">
            <template #suffix><span class="unit">ms</span></template>
          </n-statistic>
        </div>
      </n-grid-item>
    </n-grid>

    <!-- 图表区：撑满剩余高度，页面不滚动 -->
    <div class="charts-area">
      <div v-if="!canQuery" class="empty-hint">请选择策略和 IP 后查看历史数据</div>
      <div class="glass-card strip-panel">
        <n-tabs
          v-if="dayTabs.length > 0"
          v-model:value="currentDayIndex"
          size="small"
          type="segment"
          class="day-tabs"
        >
          <n-tab-pane v-for="(day, idx) in dayTabs" :key="idx" :name="idx" :tab="day" />
        </n-tabs>
        <div ref="stripChartRef" class="chart-host"></div>
      </div>
      <div class="bottom-row">
        <div class="glass-card trend-panel">
          <div ref="trendChartRef" class="chart-host"></div>
        </div>
        <div class="glass-card pie-panel">
          <div ref="pieChartRef" class="chart-host"></div>
        </div>
      </div>
    </div>

    <!-- 分钟详情弹窗 -->
    <n-modal
      v-model:show="showMinuteModal"
      preset="card"
      :title="`分钟详情 ${activeWindow?.time ? fmtDateTime(activeWindow.time) : ''}`"
      style="width: 680px"
    >
      <n-descriptions :column="4" label-placement="left" bordered size="small">
        <n-descriptions-item label="状态" :span="2">
          <n-tag :type="statusTagType(activeWindow?.status)" size="small" round>
            {{ statusText(activeWindow?.status ?? -1) }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="平均延迟" :span="2">
          {{ activeWindow?.avgLatencyMs ?? '-' }} ms
        </n-descriptions-item>
        <n-descriptions-item label="稳定">{{ activeWindow?.onlineCount ?? 0 }} 次</n-descriptions-item>
        <n-descriptions-item label="不稳定">{{ activeWindow?.unstableCount ?? 0 }} 次</n-descriptions-item>
        <n-descriptions-item label="断联">{{ activeWindow?.offlineCount ?? 0 }} 次</n-descriptions-item>
        <n-descriptions-item label="总数">{{ activeWindow?.total ?? 0 }} 次</n-descriptions-item>
      </n-descriptions>

      <!-- 该分钟内每条探测记录的延迟曲线：稳定绿 / 不稳定黄 / 超时红（断联画在 -1） -->
      <n-spin :show="detailLoading">
        <div v-if="detailRecords.length > 0" ref="minuteChartRef" style="width: 100%; height: 260px; margin-top: 12px;"></div>
        <div v-else class="minute-empty">该分钟内没有探测记录</div>
      </n-spin>
    </n-modal>
  </div>
</template>

<style scoped>
.analysis-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 104px);
}

.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
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
  border-left: 4px solid #3b82f6;
  transition: all 0.3s ease;
  padding: 12px 16px;
}
.stat-card:hover {
  transform: translateY(-2px);
}
.stat-card.blue { border-left-color: #3b82f6; }
.stat-card.purple { border-left-color: #8b5cf6; }
.stat-card.green { border-left-color: #10b981; }
.stat-card.yellow { border-left-color: #f0a020; }
.stat-card.red { border-left-color: #d03050; }

.unit {
  font-size: 13px;
  opacity: 0.65;
  margin-left: 4px;
}

/* 图表区：状态条固定高，下方趋势图 + 饼图按比例撑满剩余空间 */
.charts-area {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 16px;
  position: relative;
}

.empty-hint {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  opacity: 0.45;
  pointer-events: none;
}

.strip-panel {
  flex: 0 0 200px;
  min-height: 160px;
  padding: 8px 16px;
  display: flex;
  flex-direction: column;
}

.day-tabs {
  flex: 0 0 auto;
  margin-bottom: 4px;
}

.bottom-row {
  flex: 1;
  min-height: 220px;
  display: flex;
  gap: 16px;
}

.trend-panel {
  flex: 2;
  min-width: 0;
  padding: 8px 16px;
}

.pie-panel {
  flex: 1;
  min-width: 0;
  padding: 8px 16px;
}

.chart-host {
  width: 100%;
  height: 100%;
}

.minute-empty {
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.5;
  font-size: 13px;
}

/* 窄屏回退：图表上下堆叠，允许滚动 */
@media (max-width: 1100px) {
  .analysis-container {
    height: auto;
  }
  .bottom-row {
    flex-direction: column;
  }
  .trend-panel,
  .pie-panel {
    flex: none;
    height: 320px;
  }
}
</style>
