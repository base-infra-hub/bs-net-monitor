package ip

import (
	"strconv"
	"sync"

	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"

	"github.com/gin-gonic/gin"
)

// Handler 处理 IP 相关的 HTTP 请求。
type Handler struct {
	svc *service.IPService
}

var (
	handlerInstance *Handler
	handlerOnce     sync.Once
)

// GetHandler 返回 IP Handler 的单例。
func GetHandler() *Handler {
	handlerOnce.Do(func() {
		handlerInstance = &Handler{
			svc: service.GetIPService(),
		}
	})
	return handlerInstance
}

func (h *Handler) List(c *gin.Context) {
	var query dto.IPListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pageRes, err := h.svc.List(middleware.TenantFromContext(c), query)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, pageRes)
}

func (h *Handler) Create(c *gin.Context) {
	var req dto.IPCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ip, err := h.svc.Create(middleware.TenantFromContext(c), &req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, ip)
}

func (h *Handler) Get(c *gin.Context) {
	ipId, err := strconv.ParseUint(c.Param("ipId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ipId 无效")
		return
	}

	ip, err := h.svc.Get(middleware.TenantFromContext(c), ipId)
	if err != nil {
		response.NotFound(c, "IP 不存在")
		return
	}
	response.OK(c, ip)
}

func (h *Handler) Update(c *gin.Context) {
	ipId, err := strconv.ParseUint(c.Param("ipId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ipId 无效")
		return
	}

	var req dto.IPUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ip, err := h.svc.Update(middleware.TenantFromContext(c), ipId, &req)
	if err != nil {
		if err == service.ErrIPNotFound {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, ip)
}

func (h *Handler) Delete(c *gin.Context) {
	ipId, err := strconv.ParseUint(c.Param("ipId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ipId 无效")
		return
	}

	if err := h.svc.Delete(middleware.TenantFromContext(c), ipId); err != nil {
		if err == service.ErrIPNotFound {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}
