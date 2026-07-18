package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"bs-net-monitor/internal/constant"
	"bs-net-monitor/pkg/auth"
	"bs-net-monitor/pkg/response"
)

const (
	// SessionContextKey 存储在 gin 上下文中的 Session 键名。
	SessionContextKey = "auth_session"
	// JWTClaimsContextKey 存储在 gin 上下文中的 JWT claims 键名。
	JWTClaimsContextKey = "auth_jwt_claims"
)

// CORSMiddleware 处理跨域请求，支持携带 Cookie 和自定义请求头。
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// WebAuthMiddleware 双轨鉴权：优先校验 Bearer JWT（纯本地 CPU 解密，极速），其次校验 Session Cookie/Authorization（需要查 Redis，慢）。
func WebAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		cookieSessionID, _ := c.Cookie(auth.SessionCookieName)

		var jwtToken string
		var sessionID string

		// 判断 Authorization 里的 Token 是 JWT 还是 Session ID
		// JWT 格式为 header.payload.signature，必定含有 2 个点
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.Count(token, ".") == 2 {
				jwtToken = token
			} else {
				sessionID = token
			}
		}

		// 优先以 Cookie 里的 Session ID 为准
		if cookieSessionID != "" {
			sessionID = cookieSessionID
		}

		log.Printf("[WebAuth] 开始鉴权 - Path: %s, Header长度: %d, Cookie有无: %t, 识别为JWT: %t, 识别为Session: %t",
			c.Request.URL.Path, len(authHeader), cookieSessionID != "", jwtToken != "", sessionID != "")

		// 轨道 1：JWT 鉴权（无 Redis I/O，速度最快，优先处理）
		if jwtToken != "" {
			claims, err := auth.VerifyJWT(jwtToken)
			if err == nil {
				// 租户 ID 必须内置在 JWT claims 中，禁止从请求头等外部渠道获取
				tenantID, ok := claims["tenant_id"].(string)
				if !ok || tenantID == "" {
					log.Printf("[WebAuth] JWT claims 缺少 tenant_id - 返回 401")
					response.Error(c, http.StatusUnauthorized, "JWT 缺少租户信息")
					c.Abort()
					return
				}
				log.Printf("[WebAuth] JWT 鉴权成功 - 租户: %s", tenantID)
				c.Set(JWTClaimsContextKey, claims)
				c.Set(constant.TenantContextKey, tenantID)
				c.Next()
				return
			} else {
				log.Printf("[WebAuth] JWT 鉴权失败: %v", err)
			}
		}

		// 轨道 2：Session 鉴权（需要查 Redis 缓存，较慢）
		if sessionID != "" {
			sess, err := auth.GetSession(sessionID)
			if err == nil {
				log.Printf("[WebAuth] Session 鉴权成功 - 用户: %s, 租户: %s", sess.Username, sess.TenantID)
				c.Set(SessionContextKey, sess)
				c.Set(constant.TenantContextKey, sess.TenantID)
				c.Next()
				return
			} else {
				log.Printf("[WebAuth] Session 鉴权失败: %v", err)
			}
		}

		log.Printf("[WebAuth] 鉴权完全失败 - 返回 401")
		response.Error(c, http.StatusUnauthorized, "登录状态已失效，请重新登录")
		c.Abort()
	}
}

// SessionAuthMiddleware 单轨鉴权：只允许 Session 鉴权（从 Cookie 或 Authorization 携带 Session ID），不支持 JWT。
func SessionAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookieSessionID, _ := c.Cookie(auth.SessionCookieName)
		var sessionID string

		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			// 过滤掉 JWT，只有不是 JWT 时才判定为 Session ID，避免查 Redis 浪费时间
			if strings.Count(token, ".") != 2 {
				sessionID = token
			} else {
				log.Printf("[SessionAuth] 忽略传入的 JWT 令牌 (此路由仅支持 Session): Path: %s", c.Request.URL.Path)
			}
		}

		if cookieSessionID != "" {
			sessionID = cookieSessionID
		}

		log.Printf("[SessionAuth] 开始鉴权 - Path: %s, Cookie有无: %t, SessionID有无: %t",
			c.Request.URL.Path, cookieSessionID != "", sessionID != "")

		if sessionID != "" {
			sess, err := auth.GetSession(sessionID)
			if err == nil {
				log.Printf("[SessionAuth] Session 鉴权成功 - 用户: %s, 租户: %s", sess.Username, sess.TenantID)
				c.Set(SessionContextKey, sess)
				c.Set(constant.TenantContextKey, sess.TenantID)
				c.Next()
				return
			} else {
				log.Printf("[SessionAuth] Session 鉴权失败: %v", err)
			}
		}

		log.Printf("[SessionAuth] 鉴权完全失败 - 返回 401")
		response.Error(c, http.StatusUnauthorized, "登录状态已失效，请重新登录")
		c.Abort()
	}
}

// TenantFromContext 从 gin 上下文中获取租户 ID。
func TenantFromContext(c *gin.Context) string {
	val, exists := c.Get(constant.TenantContextKey)
	if !exists {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}

// SessionFromContext 从 gin 上下文中获取 Session。
func SessionFromContext(c *gin.Context) (*auth.Session, bool) {
	val, exists := c.Get(SessionContextKey)
	if !exists {
		return nil, false
	}
	sess, ok := val.(*auth.Session)
	if !ok {
		return nil, false
	}
	return sess, true
}

// JWTClaimsFromContext 从 gin 上下文中获取 JWT claims。
func JWTClaimsFromContext(c *gin.Context) (map[string]any, bool) {
	val, exists := c.Get(JWTClaimsContextKey)
	if !exists {
		return nil, false
	}
	claims, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return claims, true
}
