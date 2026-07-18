package tenant

import (
	"sync"

	"github.com/gin-gonic/gin"

	"bs-net-monitor/internal/service"
	"bs-net-monitor/pkg/auth"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"
)

// Handler 处理租户相关的 HTTP 请求。
type Handler struct {
	svc *service.TacticsService
}

var (
	handlerInstance *Handler
	handlerOnce     sync.Once
)

// GetHandler 返回租户 Handler 的单例。
func GetHandler() *Handler {
	handlerOnce.Do(func() {
		handlerInstance = &Handler{
			svc: service.GetTacticsService(),
		}
	})
	return handlerInstance
}

func (h *Handler) List(c *gin.Context) {
	tenants, err := h.svc.ListTenants()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tenants)
}

// SwitchRequest 切换租户请求参数。
type SwitchRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
}

// Switch 切换当前 Session 的租户：把 tenant_id 写回 Redis 中的 session 记录，
// 后续请求的租户上下文均由 WebAuthMiddleware 从 Session 注入。Cookie 无需变更。
func (h *Handler) Switch(c *gin.Context) {
	var req SwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "缺少 tenant_id 参数")
		return
	}

	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		response.Unauthorized(c, "登录状态已失效，请重新登录")
		return
	}

	if err := auth.UpdateSessionTenant(sess.SessionID, req.TenantID); err != nil {
		response.InternalError(c, "切换租户失败")
		return
	}

	response.OKMsg(c, "切换成功", gin.H{"tenantId": req.TenantID})
}
