package ip

import (
	"errors"
	"sync"

	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"

	"github.com/gin-gonic/gin"
)

// HistoryHandler 处理 IP 历史数据聚合查询的 HTTP 请求。
type HistoryHandler struct {
	svc *service.HistoryService
}

var (
	historyHandlerInstance *HistoryHandler
	historyHandlerOnce     sync.Once
)

// GetHistoryHandler 返回历史查询 Handler 的单例。
func GetHistoryHandler() *HistoryHandler {
	historyHandlerOnce.Do(func() {
		historyHandlerInstance = &HistoryHandler{
			svc: service.GetHistoryService(),
		}
	})
	return historyHandlerInstance
}

// Query 按日期范围聚合查询连接状态：GET /api/v1/ips/history?startDate=2006-01-02&endDate=2006-01-08&tacticsId=1&ipId=2
func (h *HistoryHandler) Query(c *gin.Context) {
	var query dto.HistoryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	res, err := h.svc.QueryRange(middleware.TenantFromContext(c), query)
	if err != nil {
		if errors.Is(err, service.ErrInvalidDate) || errors.Is(err, service.ErrInvalidRange) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, res)
}

// Detail 查询某一分钟窗口内的探测明细：GET /api/v1/ips/history/detail?time=2026-07-18T15%3A04%3A05%2B08%3A00&tacticsId=1&ipId=2
// time 支持 2006-01-02 15:04:05 或 RFC3339 带时区格式。
func (h *HistoryHandler) Detail(c *gin.Context) {
	var query dto.HistoryDetailQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	records, err := h.svc.QueryMinuteDetail(middleware.TenantFromContext(c), query)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTime) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, records)
}
