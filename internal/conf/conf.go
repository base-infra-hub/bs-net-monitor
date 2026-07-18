package conf

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"bs-net-monitor/pkg/logger"
)

// Config 全局配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Ticket   TicketConfig   `yaml:"ticket"`
	Auth     AuthConfig     `yaml:"auth"`
	WS       WSConfig       `yaml:"ws"`
	Log      logger.Config  `yaml:"log"`
}

// WSConfig WebSocket 推送配置
type WSConfig struct {
	StatisticsPushIntervalMs int `yaml:"statistics_push_interval_ms"`
	RealtimePushIntervalMs   int `yaml:"realtime_push_interval_ms"`
}

// StatisticsPushInterval 返回 statistics 推送间隔，默认 1 秒
func (c WSConfig) StatisticsPushInterval() time.Duration {
	if c.StatisticsPushIntervalMs <= 0 {
		return time.Second
	}
	return time.Duration(c.StatisticsPushIntervalMs) * time.Millisecond
}

// RealtimePushInterval 返回 realtime 推送间隔，默认 1 秒
func (c WSConfig) RealtimePushInterval() time.Duration {
	if c.RealtimePushIntervalMs <= 0 {
		return time.Second
	}
	return time.Duration(c.RealtimePushIntervalMs) * time.Millisecond
}

// AuthConfig 鉴权配置（Session + JWT 双轨）
type AuthConfig struct {
	RSAPublicKey string      `yaml:"rsa_public_key"`
	ServiceTag   string      `yaml:"service_tag"`
	Admin        AdminConfig `yaml:"admin"`
}

// AdminConfig Web 后台管理员账号与登录会话配置
type AdminConfig struct {
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	SessionTTLSeconds int    `yaml:"session_ttl_seconds"`
}

// SessionTTL 返回 session 过期时间，默认 24 小时
func (c AdminConfig) SessionTTL() time.Duration {
	if c.SessionTTLSeconds <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(c.SessionTTLSeconds) * time.Second
}

// ServerConfig 服务端口配置
type ServerConfig struct {
	HTTPPort int `yaml:"http_port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// TicketConfig Ticket 加密配置
type TicketConfig struct {
	AESKey        string `yaml:"aes_key"`
	ExpireSeconds int    `yaml:"expire_seconds"`
}

// PostgresConfig Postgres 详细配置
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

func (c PostgresConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// LoadConfig 加载配置，启动时调用
func LoadConfig() (*Config, error) {
	var loadErr error
	configOnce.Do(func() {
		globalConfig = &Config{}
		if err := globalConfig.load(); err != nil {
			loadErr = err
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return globalConfig, nil
}

// GetConfig 获取已加载的配置单例
func GetConfig() *Config {
	if globalConfig == nil {
		log.Fatal("[配置错误] 配置尚未加载，请先调用 LoadConfig")
	}
	return globalConfig
}

// PrintConfig 打印当前配置（敏感字段脱敏）
func (c *Config) PrintConfig() {
	log.Println("┌─────────────────────────── 当前配置 ───────────────────────────")
	log.Printf("│ server.http_port           : %d", c.Server.HTTPPort)
	log.Printf("│ database.postgres.host     : %s", c.Database.Postgres.Host)
	log.Printf("│ database.postgres.port     : %d", c.Database.Postgres.Port)
	log.Printf("│ database.postgres.user     : %s", c.Database.Postgres.User)
	log.Printf("│ database.postgres.password : %s", maskSecret(c.Database.Postgres.Password))
	log.Printf("│ database.postgres.dbname   : %s", c.Database.Postgres.DBName)
	log.Printf("│ redis.addr                 : %s", c.Redis.Addr)
	log.Printf("│ redis.password             : %s", maskSecret(c.Redis.Password))
	log.Printf("│ redis.db                   : %d", c.Redis.DB)
	log.Printf("│ ticket.expire_seconds      : %d", c.Ticket.ExpireSeconds)
	log.Printf("│ auth.rsa_public_key        : %s", maskSecret(c.Auth.RSAPublicKey))
	log.Printf("│ auth.service_tag           : %s", c.Auth.ServiceTag)
	log.Printf("│ auth.admin.username        : %s", c.Auth.Admin.Username)
	log.Printf("│ auth.admin.password        : %s", maskSecret(c.Auth.Admin.Password))
	log.Printf("│ auth.admin.session_ttl_seconds : %d", c.Auth.Admin.SessionTTLSeconds)
	log.Printf("│ ws.statistics_push_interval_ms : %d", c.WS.StatisticsPushIntervalMs)
	log.Printf("│ ws.realtime_push_interval_ms   : %d", c.WS.RealtimePushIntervalMs)
	log.Printf("│ log.path                       : %s", c.Log.Path)
	log.Printf("│ log.max_size_mb                : %d", c.Log.MaxSize)
	log.Printf("│ log.max_age_days               : %d", c.Log.MaxAge)
	log.Printf("│ log.max_backups                : %d", c.Log.MaxBackups)
	log.Printf("│ log.stdout                     : %t", c.Log.Stdout)
	log.Printf("│ log.compress                   : %t", c.Log.Compress)
	log.Println("└────────────────────────────────────────────────────────────────")
}

func maskSecret(s string) string {
	if len(s) == 0 {
		return "(空)"
	}
	if len(s) <= 2 {
		return "**"
	}
	return s[:2] + "**"
}

// load 加载配置：只读取可执行文件所在目录下的 config.yaml，不存在或解析失败则报错。
func (c *Config) load() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}

	configPath := filepath.Join(filepath.Dir(exePath), "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件 %s 失败: %w", configPath, err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("解析配置文件 %s 失败: %w", configPath, err)
	}

	// 填充日志等默认值
	c.Log.Normalize()

	log.Printf("[配置] 已加载配置文件: %s", configPath)
	return c.validate()
}

// validate 校验配置项的合法性。
func (c *Config) validate() error {
	keyLen := len(c.Ticket.AESKey)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return fmt.Errorf("ticket.aes_key 长度必须是 16、24 或 32 字节（对应 AES-128/192/256），当前为 %d 字节", keyLen)
	}
	if c.Auth.ServiceTag == "" {
		return fmt.Errorf("auth.service_tag 不得为空，必须填写本服务的 JWT tag 标识（如 \"BS-Net-Monitor\"），防止其他服务的 JWT 越权访问")
	}
	return nil
}
