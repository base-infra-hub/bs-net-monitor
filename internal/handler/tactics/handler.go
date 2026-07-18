package tactics

import (
	"strconv"
	"sync"

	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Handler 处理策略组相关的 HTTP 请求。
type Handler struct {
	svc *service.TacticsService
}

var (
	handlerInstance *Handler
	handlerOnce     sync.Once
)

// GetHandler 返回策略组 Handler 的单例。
func GetHandler() *Handler {
	handlerOnce.Do(func() {
		handlerInstance = &Handler{
			svc: service.GetTacticsService(),
		}
	})
	return handlerInstance
}

func (h *Handler) List(c *gin.Context) {
	list, err := h.svc.List(middleware.TenantFromContext(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *Handler) Create(c *gin.Context) {
	var req dto.TacticsCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tactics, err := h.svc.Create(middleware.TenantFromContext(c), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tactics)
}

func (h *Handler) Get(c *gin.Context) {
	tacticsId, err := strconv.ParseUint(c.Param("tacticsId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "tacticsId 无效")
		return
	}

	tactics, err := h.svc.Get(middleware.TenantFromContext(c), tacticsId)
	if err != nil {
		response.NotFound(c, "策略组不存在")
		return
	}
	response.OK(c, tactics)
}

func (h *Handler) Update(c *gin.Context) {
	tacticsId, err := strconv.ParseUint(c.Param("tacticsId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "tacticsId 无效")
		return
	}

	var req dto.TacticsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tactics, err := h.svc.Update(middleware.TenantFromContext(c), tacticsId, &req)
	if err != nil {
		if err == service.ErrTacticsNotFound {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tactics)
}

func (h *Handler) Delete(c *gin.Context) {
	tacticsId, err := strconv.ParseUint(c.Param("tacticsId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "tacticsId 无效")
		return
	}

	if err := h.svc.Delete(middleware.TenantFromContext(c), tacticsId); err != nil {
		if err == service.ErrTacticsNotFound {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}
