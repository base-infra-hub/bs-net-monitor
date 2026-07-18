package ip

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"bs-net-monitor/internal/conf"
	"bs-net-monitor/internal/detector"
	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// LiveHandler 处理 IP 实时状态相关的 WebSocket 与 HTTP 接口。
type LiveHandler struct {
	ticketSvc *service.LiveTicketService
	manager   *detector.Manager
	mu        sync.RWMutex
	conns     map[string]*liveConn
	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

type liveConn struct {
	connId   string
	tenantId string
	filter   *filterOption
	ws       *websocket.Conn
	sendMu   sync.Mutex
	sendCh   chan wsMessage
	done     chan struct{}
}

type filterOption struct {
	all       bool
	tacticsId uint64
	ipIds     map[uint64]struct{}
}

type wsMessage struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type statisticData struct {
	Online   int `json:"online"`
	Offline  int `json:"offline"`
	Unstable int `json:"unstable"`
}

type realtimeFilterMsg struct {
	Action    string   `json:"action"`
	IpIds     []uint64 `json:"ipIds,omitempty"`
	TacticsId *uint64  `json:"tacticsId,omitempty"`
	All       bool     `json:"all,omitempty"`
}

var (
	liveHandlerInstance *LiveHandler
	liveHandlerOnce     sync.Once
)

// GetLiveHandler 返回 LiveHandler 单例。
func GetLiveHandler() *LiveHandler {
	liveHandlerOnce.Do(func() {
		liveHandlerInstance = &LiveHandler{
			ticketSvc: service.GetLiveTicketService(),
			manager:   detector.GetManager(),
			conns:     make(map[string]*liveConn),
			stopCh:    make(chan struct{}),
		}
	})
	return liveHandlerInstance
}

// Start 启动 LiveHandler 的后台 goroutine。
func (h *LiveHandler) Start() {
	h.startOnce.Do(func() {
		go h.broadcastStatistics()
		go h.broadcastRealtime()
	})
}

// Stop 停止 LiveHandler 的后台 goroutine。
func (h *LiveHandler) Stop() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
		h.mu.Lock()
		for _, c := range h.conns {
			c.close()
		}
		h.conns = make(map[string]*liveConn)
		h.mu.Unlock()
	})
}

// ApplyTicket 业务服务申请 Ticket。
func (h *LiveHandler) ApplyTicket(c *gin.Context) {
	tenantId := middleware.TenantFromContext(c)
	if tenantId == "" {
		response.BadRequest(c, "缺少租户信息")
		return
	}

	ticket, expire, err := h.ticketSvc.ApplyTicket(tenantId)
	if err != nil {
		log.Printf("[live] 申请 Ticket 失败 (租户: %s): %v", tenantId, err)
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"ticket":        ticket,
		"expireSeconds": expire,
	})
}

// Statistic 返回当前租户下 IP 的在线/离线/不稳定统计。
func (h *LiveHandler) Statistic(c *gin.Context) {
	tenantId := middleware.TenantFromContext(c)
	online, offline, unstable := h.manager.Statistic(tenantId)
	response.OK(c, statisticData{Online: online, Offline: offline, Unstable: unstable})
}

// Subscribe WebSocket 实时订阅入口（/ips/live/subscribe）。
func (h *LiveHandler) Subscribe(c *gin.Context) {
	ticketStr := c.Query("ticket")
	liveTicket, err := h.ticketSvc.ValidateTicket(ticketStr)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[live] WebSocket 升级失败: %v", err)
		return
	}

	conn := &liveConn{
		connId:   service.GenerateConnId(),
		tenantId: liveTicket.TenantId,
		ws:       ws,
		sendCh:   make(chan wsMessage, 64),
		done:     make(chan struct{}),
	}

	h.addConn(conn)
	go conn.writeLoop()
	go conn.readLoop(h)
}

func (h *LiveHandler) addConn(c *liveConn) {
	h.mu.Lock()
	h.conns[c.connId] = c
	h.mu.Unlock()
}

func (h *LiveHandler) removeConn(id string) {
	h.mu.Lock()
	c, ok := h.conns[id]
	if ok {
		delete(h.conns, id)
	}
	h.mu.Unlock()
	if ok {
		c.close()
	}
}

func (h *LiveHandler) broadcastStatistics() {
	ticker := time.NewTicker(conf.GetConfig().WS.StatisticsPushInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			conns := make([]*liveConn, 0, len(h.conns))
			for _, c := range h.conns {
				conns = append(conns, c)
			}
			h.mu.RUnlock()
			stats := make(map[string]statisticData)
			for _, c := range conns {
				if _, exists := stats[c.tenantId]; exists {
					continue
				}
				online, offline, unstable := h.manager.Statistic(c.tenantId)
				stats[c.tenantId] = statisticData{Online: online, Offline: offline, Unstable: unstable}
			}
			for _, c := range conns {
				c.send(wsMessage{Type: "statistics", Data: stats[c.tenantId]})
			}
		case <-h.stopCh:
			return
		}
	}
}

func (h *LiveHandler) broadcastRealtime() {
	ticker := time.NewTicker(conf.GetConfig().WS.RealtimePushInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			conns := make([]*liveConn, 0, len(h.conns))
			for _, c := range h.conns {
				conns = append(conns, c)
			}
			h.mu.RUnlock()

			for _, c := range conns {
				if c.filter == nil {
					continue
				}
				var statuses []dto.PingStatusVO
				switch {
				case c.filter.tacticsId > 0:
					statuses = h.manager.LatestStatusesByTactics(c.tenantId, c.filter.tacticsId)
				case len(c.filter.ipIds) > 0:
					statuses = make([]dto.PingStatusVO, 0, len(c.filter.ipIds))
					for ipId := range c.filter.ipIds {
						if vo, ok := h.manager.LatestStatus(ipId); ok {
							statuses = append(statuses, vo)
						}
					}
				default:
					statuses = h.manager.LatestStatuses(c.tenantId)
				}
				if len(statuses) > 0 {
					c.send(wsMessage{Type: "realtime", Data: statuses})
				}
			}
		case <-h.stopCh:
			return
		}
	}
}

func (c *liveConn) readLoop(h *LiveHandler) {
	defer h.removeConn(c.connId)

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}

		var msg realtimeFilterMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Action {
		case "realtimeFilter":
			c.handleRealtimeFilter(msg)
		}
	}
}

func (c *liveConn) handleRealtimeFilter(msg realtimeFilterMsg) {
	filter := &filterOption{}
	if msg.All {
		filter.all = true
	}
	if msg.TacticsId != nil {
		filter.tacticsId = *msg.TacticsId
	}
	if len(msg.IpIds) > 0 {
		filter.ipIds = make(map[uint64]struct{}, len(msg.IpIds))
		for _, id := range msg.IpIds {
			filter.ipIds[id] = struct{}{}
		}
	}

	c.filter = filter
}

func (c *liveConn) writeLoop() {
	defer c.close()
	for {
		select {
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			c.sendMu.Lock()
			err := c.ws.WriteJSON(msg)
			c.sendMu.Unlock()
			if err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *liveConn) send(msg wsMessage) {
	select {
	case c.sendCh <- msg:
	default:
	}
}

func (c *liveConn) close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	c.ws.Close()
}
