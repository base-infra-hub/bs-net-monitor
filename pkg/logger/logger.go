package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置。
type Config struct {
	Path       string `yaml:"path"`         // 日志文件路径，相对路径基于当前工作目录；留空则只输出到控制台
	MaxSize    int    `yaml:"max_size_mb"`  // 单个日志文件最大大小（MB），超过则轮转
	MaxAge     int    `yaml:"max_age_days"` // 日志文件保留天数
	MaxBackups int    `yaml:"max_backups"`  // 保留的旧文件数量
	Stdout     bool   `yaml:"stdout"`       // 是否同时输出到控制台
	Compress   bool   `yaml:"compress"`     // 是否压缩旧日志
}

// Normalize 为未配置项填充默认值。
func (c *Config) Normalize() {
	if c.Path == "" {
		c.Path = "log/app.log"
	}
	if c.MaxSize <= 0 {
		c.MaxSize = 10
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 7
	}
	if c.MaxBackups <= 0 {
		c.MaxBackups = 5
	}
}

// Init 初始化日志输出：按配置写入文件，并按需同时输出到控制台。
func Init(cfg Config) error {
	if cfg.Path == "" {
		return nil
	}

	// 相对路径基于当前工作目录解析（部署时通常就是 exe 所在目录）
	path := cfg.Path
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = filepath.Join(wd, path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	w := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxSize,
		MaxAge:     cfg.MaxAge,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	var out io.Writer = w
	if cfg.Stdout {
		out = io.MultiWriter(os.Stdout, w)
	}
	log.SetOutput(out)
	return nil
}
