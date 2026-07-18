package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"bs-net-monitor/internal/conf"
	rdb "bs-net-monitor/pkg/redis"
)

// LiveTicket 是 WebSocket 实时连接的凭证内容。
type LiveTicket struct {
	TenantId string `json:"tenantId"`
}

// LiveTicketService 负责实时连接 Ticket 的申请与校验。
type LiveTicketService struct {
	cfg       *conf.TicketConfig
	redis     *redis.Client
	aesGCM    cipher.AEAD
	keyHashMu sync.RWMutex
}

var (
	liveTicketServiceInstance *LiveTicketService
	liveTicketServiceOnce     sync.Once
)

// GetLiveTicketService 返回 Ticket 服务单例。
func GetLiveTicketService() *LiveTicketService {
	liveTicketServiceOnce.Do(func() {
		cfg := conf.GetConfig().Ticket
		block, err := aes.NewCipher([]byte(cfg.AESKey))
		if err != nil {
			panic(fmt.Sprintf("初始化 Ticket AES 失败: %v", err))
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			panic(fmt.Sprintf("初始化 Ticket AES-GCM 失败: %v", err))
		}
		liveTicketServiceInstance = &LiveTicketService{
			cfg:    &cfg,
			redis:  rdb.GetClient(),
			aesGCM: gcm,
		}
	})
	return liveTicketServiceInstance
}

// ApplyTicket 为指定租户申请一个一次性 Ticket。
func (s *LiveTicketService) ApplyTicket(tenantId string) (string, int, error) {
	if tenantId == "" {
		return "", 0, errors.New("tenantId 不能为空")
	}

	ticket := LiveTicket{TenantId: tenantId}
	data, err := json.Marshal(ticket)
	if err != nil {
		return "", 0, err
	}

	nonce := make([]byte, s.aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", 0, err
	}

	cipherText := s.aesGCM.Seal(nonce, nonce, data, nil)
	ticketStr := base64.URLEncoding.EncodeToString(cipherText)

	expire := s.cfg.ExpireSeconds
	if expire <= 0 {
		expire = 60
	}

	ctx := context.Background()
	if err := s.redis.Set(ctx, ticketKey(ticketStr), 1, time.Duration(expire)*time.Second).Err(); err != nil {
		return "", 0, err
	}

	return ticketStr, expire, nil
}

// ValidateTicket 校验 Ticket，成功后返回 LiveTicket。
func (s *LiveTicketService) ValidateTicket(ticketStr string) (*LiveTicket, error) {
	if ticketStr == "" {
		return nil, errors.New("ticket 不能为空")
	}

	ctx := context.Background()
	// 用后即焚
	val, err := s.redis.GetDel(ctx, ticketKey(ticketStr)).Result()
	if errors.Is(err, redis.Nil) || val == "" {
		return nil, errors.New("ticket 不存在或已失效")
	}
	if err != nil {
		return nil, err
	}

	cipherText, err := base64.URLEncoding.DecodeString(ticketStr)
	if err != nil {
		return nil, errors.New("ticket 格式错误")
	}

	nonceSize := s.aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, errors.New("ticket 长度错误")
	}

	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	plain, err := s.aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, errors.New("ticket 解密失败")
	}

	var liveTicket LiveTicket
	if err := json.Unmarshal(plain, &liveTicket); err != nil {
		return nil, errors.New("ticket 内容错误")
	}

	return &liveTicket, nil
}

func ticketKey(ticket string) string {
	return rdb.WrapKey("ticket:" + ticket)
}

// GenerateConnId 生成唯一的连接 ID。
func GenerateConnId() string {
	return uuid.New().String()
}
