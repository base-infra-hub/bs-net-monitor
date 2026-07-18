package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"bs-net-monitor/internal/conf"
	"bs-net-monitor/pkg/auth"
	"bs-net-monitor/pkg/middleware"
	"bs-net-monitor/pkg/response"
)

// LoginRequest 登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	SessionID string `json:"sessionId"`
}

// Login 校验 Web 后台账号密码，创建 Session 并写入 Cookie。
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入账号和密码")
		return
	}

	cfg := conf.GetConfig().Auth.Admin
	if req.Username != cfg.Username || req.Password != cfg.Password {
		response.Unauthorized(c, "账号或密码错误")
		return
	}

	sessionID, err := auth.CreateSession(req.Username)
	if err != nil {
		response.InternalError(c, "创建登录会话失败")
		return
	}

	// 写入 HttpOnly Cookie，显式设置 SameSite=Lax 保证同站请求自动携带
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		auth.SessionCookieName,
		sessionID,
		conf.GetConfig().Auth.Admin.SessionTTLSeconds,
		"/",
		"",
		false,
		true,
	)

	response.OK(c, LoginResponse{SessionID: sessionID})
}

// Logout 登出，删除 Session 并清空 Cookie。
func Logout(c *gin.Context) {
	sessionID, err := c.Cookie(auth.SessionCookieName)
	if err == nil && sessionID != "" {
		_ = auth.DestroySession(sessionID)
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		auth.SessionCookieName,
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
	response.OKMsg(c, "已退出登录", nil)
}

// CheckResponse 登录状态检查响应
type CheckResponse struct {
	Type     string    `json:"type"`
	Username string    `json:"username,omitempty"`
	Subject  string    `json:"subject,omitempty"`
	LoginAt  time.Time `json:"loginAt,omitempty"`
}

// Check 返回当前登录状态，供前端路由守卫使用。
func Check(c *gin.Context) {
	if sess, ok := middleware.SessionFromContext(c); ok {
		response.OK(c, CheckResponse{
			Type:     "session",
			Username: sess.Username,
			LoginAt:  sess.LoginAt,
		})
		return
	}

	if claims, ok := middleware.JWTClaimsFromContext(c); ok {
		var sub string
		if s, ok := claims["sub"].(string); ok {
			sub = s
		}
		response.OK(c, CheckResponse{
			Type:    "jwt",
			Subject: sub,
		})
		return
	}

	response.Unauthorized(c, "未登录")
}
