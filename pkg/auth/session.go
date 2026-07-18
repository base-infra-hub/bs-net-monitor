package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"bs-net-monitor/internal/conf"
	rdb "bs-net-monitor/pkg/redis"
)

const (
	SessionCookieName = "sessionId"
)

// Session 存储在 Redis 中的会话信息
type Session struct {
	SessionID string    `json:"sessionId"`
	Username  string    `json:"username"`
	TenantID  string    `json:"tenantId"`
	LoginAt   time.Time `json:"loginAt"`
}

// CreateSession 校验账号密码后为指定用户创建 Session，返回 sessionId。
// 新建 Session 的租户为空，需要前端引导用户调用租户切换接口写入。
func CreateSession(username string) (string, error) {
	cfg := conf.GetConfig()
	sessionID := uuid.New().String()

	sess := Session{
		SessionID: sessionID,
		Username:  username,
		TenantID:  "",
		LoginAt:   time.Now(),
	}

	data, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("序列化 session 失败: %w", err)
	}

	ctx := context.Background()
	if err := rdb.GetClient().Set(ctx, sessionKey(sessionID), string(data), cfg.Auth.Admin.SessionTTL()).Err(); err != nil {
		return "", fmt.Errorf("写入 session 到 redis 失败: %w", err)
	}

	return sessionID, nil
}

// GetSession 根据 sessionId 从 Redis 获取 Session。
func GetSession(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId 为空")
	}

	ctx := context.Background()
	val, err := rdb.GetClient().Get(ctx, sessionKey(sessionID)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session 不存在或已过期")
	}
	if err != nil {
		return nil, fmt.Errorf("读取 session 失败: %w", err)
	}

	var sess Session
	if err := json.Unmarshal([]byte(val), &sess); err != nil {
		return nil, fmt.Errorf("解析 session 失败: %w", err)
	}

	return &sess, nil
}

// UpdateSessionTenant 更新指定 Session 的租户 ID（管理员切换租户），保留原有剩余过期时间。
func UpdateSessionTenant(sessionID, tenantID string) error {
	sess, err := GetSession(sessionID)
	if err != nil {
		return err
	}
	sess.TenantID = tenantID

	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("序列化 session 失败: %w", err)
	}

	ctx := context.Background()
	if err := rdb.GetClient().Set(ctx, sessionKey(sessionID), string(data), redis.KeepTTL).Err(); err != nil {
		return fmt.Errorf("更新 session 租户失败: %w", err)
	}
	return nil
}

// DestroySession 删除指定 session。
func DestroySession(sessionID string) error {
	if sessionID == "" {
		return nil
	}

	ctx := context.Background()
	if err := rdb.GetClient().Del(ctx, sessionKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("删除 session 失败: %w", err)
	}
	return nil
}

func sessionKey(sessionID string) string {
	return rdb.WrapKey("session:" + sessionID)
}
