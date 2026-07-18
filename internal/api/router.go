package api

import (
	"bs-net-monitor/internal/handler/auth"
	"bs-net-monitor/internal/handler/ip"
	"bs-net-monitor/internal/handler/tactics"
	"bs-net-monitor/internal/handler/tenant"
	"bs-net-monitor/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	// 启用跨域中间件
	r.Use(middleware.CORSMiddleware())

	// 禁用自动斜杠重定向，避免 /web/login 被 301 到 /web/login/ 导致前端 history 路由异常
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	ipHandler := ip.GetHandler()
	ipBatchHandler := ip.GetBatchHandler()
	ipImportHandler := ip.GetImportHandler()
	ipLiveHandler := ip.GetLiveHandler()
	ipHistoryHandler := ip.GetHistoryHandler()
	tacticsHandler := tactics.GetHandler()
	tenantHandler := tenant.GetHandler()

	// WebSocket 入口不需要 Authorization 请求头（因为浏览器 WebSocket 无法自定义请求头，通过 Ticket 鉴权）
	r.GET("/api/v1/ips/live/subscribe", ipLiveHandler.Subscribe)

	// Web 后台登录接口，公开访问
	r.POST("/api/v1/login", auth.Login)

	// 需要登录但不需要租户 ID 的通用接口
	authGroup := r.Group("/api/v1")
	authGroup.Use(middleware.WebAuthMiddleware())
	{
		authGroup.POST("/logout", auth.Logout)
		authGroup.GET("/auth/check", auth.Check)
	}

	// 仅限 Web 后台 Session 访问的通用接口（不支持三方 JWT，安全隔离）
	sessionGroup := r.Group("/api/v1")
	sessionGroup.Use(middleware.SessionAuthMiddleware())
	{
		sessionGroup.GET("/tenants", tenantHandler.List)
		sessionGroup.POST("/tenants/switch", tenantHandler.Switch)
	}

	// 租户数据读写接口：租户 ID 由 WebAuthMiddleware 从 JWT claims 或 Session 状态中注入上下文
	v1 := r.Group("/api/v1")
	v1.Use(middleware.WebAuthMiddleware())
	{

		// IP 管理
		ips := v1.Group("/ips")
		{
			// 基础 CRUD
			ips.GET("", ipHandler.List)
			ips.POST("", ipHandler.Create)
			ips.GET("/:ipId", ipHandler.Get)
			ips.POST("/:ipId", ipHandler.Update)
			ips.DELETE("/:ipId", ipHandler.Delete)

			// 批量操作
			batch := ips.Group("/batch")
			{
				batch.POST("/update", ipBatchHandler.UpdateEnabled)
				batch.POST("/delete", ipBatchHandler.Delete)
			}

			// 导入
			ips.POST("/import", ipImportHandler.Import)
			ips.GET("/export", ipImportHandler.Export)
			ips.GET("/template", ipImportHandler.Template)

			// 历史查询：按天聚合一分钟窗口的连接状态统计；/detail 查询单分钟窗口内的探测明细
			ips.GET("/history", ipHistoryHandler.Query)
			ips.GET("/history/detail", ipHistoryHandler.Detail)

			// IP 实时状态：/ticket 申请凭证；/statistics 拉取当前统计；/subscribe 为 WebSocket 入口
			live := ips.Group("/live")
			{
				live.POST("/ticket", ipLiveHandler.ApplyTicket)
				live.GET("/statistics", ipLiveHandler.Statistic)
			}
		}

		// 策略组管理
		tacticsGroup := v1.Group("/tactics")
		{
			tacticsGroup.GET("", tacticsHandler.List)
			tacticsGroup.POST("", tacticsHandler.Create)
			tacticsGroup.GET("/:tacticsId", tacticsHandler.Get)
			tacticsGroup.POST("/:tacticsId", tacticsHandler.Update)
			tacticsGroup.DELETE("/:tacticsId", tacticsHandler.Delete)
		}
	}

	return r
}
