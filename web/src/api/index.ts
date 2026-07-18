import axios from 'axios'

const api = axios.create({
  // Web 与后端服务绑定部署，使用相对路径即可自动跟随当前 origin
  baseURL: '/api/v1',
  timeout: 10000,
  withCredentials: true,
})

api.interceptors.request.use((config) => {
  const sessionId = localStorage.getItem('web_session_id')
  if (sessionId) {
    config.headers['Authorization'] = `Bearer ${sessionId}`
  }
  return config
})

api.interceptors.response.use(
  (res) => {
    return res.data
  },
  (err) => {
    const status = err.response?.status
    const msg = err.response?.data?.msg || err.message || '请求失败'
    if (status === 401) {
      // 清理旧版本及当前会话的存储信息
      localStorage.removeItem('web_token')
      localStorage.removeItem('web_session_id')
      window.location.href = '/web/login'
      return Promise.reject(err)
    }
    window.$message?.error(msg)
    return Promise.reject(err)
  }
)

export interface Response<T = any> {
  code: number
  msg: string
  data: T
}

export interface PageRes<T> {
  total: number
  records: T[]
  current: number
  size: number
  pages: number
}

export interface IP {
  ipId: number
  tenantId: string
  name: string
  ip: string
  position: string
  remark: string
  tacticsId: number
  tacticsName: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface Tactics {
  tacticsId: number
  tenantId: string
  name: string
  intervalMs: number
  timeoutMs: number
  unstableMs: number
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface PingStatusVO {
  ipId: number
  tacticsId: number
  latencyMs: number | null
  status: number
  time: string
}

export interface LiveStatistic {
  online: number
  offline: number
  unstable: number
}

// 历史查询：一分钟聚合窗口
export interface HistoryWindow {
  time: string
  status: number // 0 断联 1 不稳定 2 稳定 -1 无数据
  latencyStatus: number // 平均延迟状态：0 超时（全部断联）1 不稳定 2 稳定 -1 无数据
  onlineCount: number
  unstableCount: number
  offlineCount: number
  total: number
  avgLatencyMs: number | null
  minLatencyMs: number | null // 该分钟内最小延迟
  maxLatencyMs: number | null // 该分钟内最大延迟
}

export interface HistorySummary {
  onlineCount: number
  unstableCount: number
  offlineCount: number
  total: number
  onlineRate: number
  avgLatencyMs: number | null
}

export interface HistoryResponse {
  startDate: string
  endDate: string
  summary: HistorySummary
  windows: HistoryWindow[]
}

// 分钟明细：一分钟窗口内的单条探测记录
export interface HistoryRecord {
  time: string
  ipId: number
  name: string
  ip: string
  latencyMs: number | null
  status: number // 0 断联 1 不稳定 2 稳定
}

export interface SubscribeFilterOptions {
  ipIds?: number[]
  tacticsId?: number
  all?: boolean
}

export interface AuthCheckResult {
  type: 'session' | 'jwt'
  username?: string
  subject?: string
  loginAt?: string
}

export const authApi = {
  login: (data: { username: string; password: string }) =>
    api.post<any, Response<{ sessionId: string }>>('/login', data),
  logout: () => api.post<any, Response<any>>('/logout', {}),
  check: () => api.get<any, Response<AuthCheckResult>>('/auth/check'),
}

export const liveApi = {
  applyTicket: () =>
    api.post<any, Response<{ ticket: string; expireSeconds: number }>>('/ips/live/ticket', {}),
  statistic: () =>
    api.get<any, Response<LiveStatistic>>('/ips/live/statistics'),
}

export const historyApi = {
  query: (params: { startDate: string; endDate?: string; tacticsId?: number; ipId?: number }) =>
    api.get<any, Response<HistoryResponse>>('/ips/history', { params }),
  detail: (params: { time: string; tacticsId?: number; ipId?: number }) =>
    api.get<any, Response<HistoryRecord[]>>('/ips/history/detail', { params }),
}

export const createLiveWebSocket = (ticket: string): WebSocket => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return new WebSocket(`${protocol}//${host}/api/v1/ips/live/subscribe?ticket=${encodeURIComponent(ticket)}`)
}

export const tenantApi = {
  list: () => api.get<any, Response<string[]>>('/tenants'),
  switch: (tenantId: string) =>
    api.post<any, Response<{ tenantId: string }>>('/tenants/switch', { tenant_id: tenantId }),
}

export const ipApi = {
  list: (params: {
    current?: number
    size?: number
    tacticsId?: number | null
    enabled?: boolean | null
  }) => api.get<any, Response<PageRes<IP>>>('/ips', { params }),
  get: (ipId: number) => api.get<any, Response<IP>>(`/ips/${ipId}`),
  create: (data: Partial<IP>) => api.post<any, Response<IP>>('/ips', data),
  update: (ipId: number, data: Partial<IP>) => api.post<any, Response<IP>>(`/ips/${ipId}`, data),
  delete: (ipId: number) => api.delete<any, Response<any>>(`/ips/${ipId}`),
  batchUpdateEnabled: (data: { ipIds: number[]; enabled: boolean }) =>
    api.post<any, Response<any>>('/ips/batch/update', data),
  batchDelete: (data: { ipIds: number[] }) =>
    api.post<any, Response<any>>('/ips/batch/delete', data),
  import: (formData: FormData) =>
    api.post<any, Response<{ imported: number }>>('/ips/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }),
  export: (tacticsId?: number | null) =>
    api.get<any, any>('/ips/export', {
      params: { tacticsId },
      responseType: 'blob',
    }),
  template: () =>
    api.get<any, any>('/ips/template', {
      responseType: 'blob',
    }),
}

export const tacticsApi = {
  list: () => api.get<any, Response<Tactics[]>>('/tactics'),
  create: (data: Partial<Tactics>) => api.post<any, Response<Tactics>>('/tactics', data),
  update: (tacticsId: number, data: Partial<Tactics>) =>
    api.post<any, Response<Tactics>>(`/tactics/${tacticsId}`, data),
  delete: (tacticsId: number) => api.delete<any, Response<any>>(`/tactics/${tacticsId}`),
}

export default api
