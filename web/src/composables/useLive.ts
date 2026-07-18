import { ref, watch, type Ref } from 'vue'
import { liveApi, createLiveWebSocket, type PingStatusVO, type LiveStatistic, type SubscribeFilterOptions } from '../api'

export { type SubscribeFilterOptions }

export function useLive(tenantId: Ref<string | undefined>, isLoginPage?: Ref<boolean>) {
  const ws = ref<WebSocket | null>(null)
  const statistic = ref<LiveStatistic>({ online: 0, offline: 0, unstable: 0 })
  const latestStatuses = ref<Map<number, PingStatusVO>>(new Map())
  const connected = ref(false)
  const currentFilter = ref<SubscribeFilterOptions>({ all: true })

  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let stopReconnect = false

  const clearReconnect = () => {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  const subscribeFilter = (opts: SubscribeFilterOptions) => {
    currentFilter.value = opts
    latestStatuses.value = new Map()
    if (ws.value?.readyState === WebSocket.OPEN) {
      ws.value.send(JSON.stringify({ action: 'realtimeFilter', ...opts }))
    }
  }

  const connect = async () => {
    if (isLoginPage?.value) {
      console.log('[live] on login page, skip connect')
      return
    }
    console.log('[live] connect called, tenantId:', tenantId.value)
    if (stopReconnect) return
    if (!tenantId.value) {
      console.log('[live] no tenantId, skip')
      return
    }

    clearReconnect()

    try {
      const res = await liveApi.applyTicket()
      const ticket = res.data.ticket
      console.log('[live] got ticket:', ticket)

      const socket = createLiveWebSocket(ticket)
      console.log('[live] connecting to:', socket.url)
      ws.value = socket

      socket.onopen = () => {
        connected.value = true
        console.log('[live] ws opened')
        socket.send(JSON.stringify({ action: 'realtimeFilter', ...currentFilter.value }))
      }

      socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'statistics') {
            statistic.value = msg.data
          } else if (msg.type === 'realtime' && msg.data) {
            if (Array.isArray(msg.data)) {
              // 每次收到后端批量状态推送时，以推送的最新集合重构 Map。
              // 这样被删除或禁用的 IP 会自动在 map 中消失，实现前端监控面板卡片的瞬间热更新下线。
              const newMap = new Map<number, PingStatusVO>()
              for (const vo of msg.data) {
                newMap.set(vo.ipId, vo)
              }
              latestStatuses.value = newMap
            } else {
              latestStatuses.value.set(msg.data.ipId, msg.data)
              latestStatuses.value = new Map(latestStatuses.value)
            }
          }
        } catch {
          // ignore invalid message
        }
      }

      socket.onclose = () => {
        connected.value = false
        ws.value = null
        if (!stopReconnect) {
          reconnectTimer = setTimeout(connect, 3000)
        }
      }

      socket.onerror = () => {
        socket.close()
      }
    } catch (err) {
      console.error('[live] apply ticket failed:', err)
      if (!stopReconnect) {
        reconnectTimer = setTimeout(connect, 3000)
      }
    }
  }

  const disconnect = () => {
    stopReconnect = true
    clearReconnect()
    ws.value?.close()
    ws.value = null
    connected.value = false
  }

  watch(
    [tenantId, () => isLoginPage?.value],
    ([newTenant, isLogin], [oldTenant, oldIsLogin]) => {
      console.log('[live] watch triggered, tenantId:', newTenant, 'isLogin:', isLogin)
      if (isLogin) {
        disconnect()
        return
      }
      if (newTenant && (newTenant !== oldTenant || oldIsLogin)) {
        disconnect()
        stopReconnect = false
        connect()
      }
    },
    { immediate: true }
  )

  return {
    connected,
    statistic,
    latestStatuses,
    subscribeFilter,
    connect,
    disconnect,
  }
}
