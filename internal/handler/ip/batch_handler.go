package ip

import (
	"sync"

	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"

	"github.com/gin-gonic/gin"
)

// BatchHandler 处理 IP 的批量操作。
type BatchHandler struct {
	svc *service.IPService
}

var (
	batchHandlerInstance *BatchHandler
	batchHandlerOnce     sync.Once
)

// GetBatchHandler 返回 IP 批量操作 Handler 的单例。
func GetBatchHandler() *BatchHandler {
	batchHandlerOnce.Do(func() {
		batchHandlerInstance = &BatchHandler{
			svc: service.GetIPService(),
		}
	})
	return batchHandlerInstance
}

func (h *BatchHandler) UpdateEnabled(c *gin.Context) {
	var req dto.IPBatchUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.BatchUpdateEnabled(middleware.TenantFromContext(c), &req); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", nil)
}

func (h *BatchHandler) Delete(c *gin.Context) {
	var req dto.IPBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.BatchDelete(middleware.TenantFromContext(c), &req); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}
