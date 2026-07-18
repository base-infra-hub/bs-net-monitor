package app

import (
	"fmt"

	"bs-net-monitor/internal/conf"
	"bs-net-monitor/internal/detector"
	"bs-net-monitor/internal/handler/ip"
	"bs-net-monitor/internal/model"
	"bs-net-monitor/internal/repository"
	"bs-net-monitor/pkg/redis"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitApp(cfg *conf.Config) error {
	dsn := cfg.Database.Postgres.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.AutoMigrate(&model.IP{}, &model.Tactics{}, &model.PingResult{}); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	// 将 ping_results 转换为 TimescaleDB 超表；若已是超表则跳过。
	if err := db.Exec("SELECT create_hypertable('ping_results', 'time', if_not_exists => true)").Error; err != nil {
		return fmt.Errorf("创建 TimescaleDB 超表失败: %w", err)
	}

	repository.Init(db)
	redis.Init(&cfg.Redis)

	if err := detector.GetManager().Start(); err != nil {
		return fmt.Errorf("启动检测引擎失败: %w", err)
	}

	ip.GetLiveHandler().Start()

	return nil
}
