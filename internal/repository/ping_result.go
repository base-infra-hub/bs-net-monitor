package repository

import (
	"sync"
	"time"

	"bs-net-monitor/internal/model"
)

// PingResultRepository 负责 ping 结果数据的读写操作。
type PingResultRepository struct{}

var (
	pingResultRepositoryInstance *PingResultRepository
	pingResultRepositoryOnce     sync.Once
)

// GetPingResultRepository 返回 ping 结果仓库的单例。
func GetPingResultRepository() *PingResultRepository {
	pingResultRepositoryOnce.Do(func() {
		pingResultRepositoryInstance = &PingResultRepository{}
	})
	return pingResultRepositoryInstance
}

func (r *PingResultRepository) Create(result *model.PingResult) error {
	return db.Create(result).Error
}

func (r *PingResultRepository) BatchCreate(results []model.PingResult) error {
	if len(results) == 0 {
		return nil
	}
	return db.CreateInBatches(results, 100).Error
}

// MinuteAggRow 一分钟时间窗口的聚合结果行。
type MinuteAggRow struct {
	Bucket            time.Time // 窗口开始时间（UTC）
	DominantTacticsId uint64    // 窗口内记录数最多的策略组，用于按策略阈值判定平均延迟的状态
	OnlineCount       int
	UnstableCount     int
	OfflineCount      int
	LatencyCount      int      // 有延迟值的记录数（断线记录延迟为 NULL，不计入）
	AvgLatencyMs      *float64 // 窗口内平均延迟（超时记录不参与），无延迟数据时为 nil
	MinLatencyMs      *float64 // 窗口内最小延迟
	MaxLatencyMs      *float64 // 窗口内最大延迟
}

// AggregateByMinute 按一分钟时间窗口聚合 [start, end) 范围内的 ping 结果。
// ipId / tacticsId 传 0 表示不按该条件过滤。
// status 取值固定为 0 断线 / 1 不稳定 / 2 稳定（定义见 internal/detector/task.go，repository 无法引用 detector 包，避免循环依赖）。
func (r *PingResultRepository) AggregateByMinute(tenantId string, start, end time.Time, ipId, tacticsId uint64) ([]MinuteAggRow, error) {
	rows := make([]MinuteAggRow, 0, 1440)
	query := db.Model(&model.PingResult{}).
		Select(`date_trunc('minute', time) AS bucket,
			mode() WITHIN GROUP (ORDER BY tactics_id) AS dominant_tactics_id,
			COUNT(*) FILTER (WHERE status = 2) AS online_count,
			COUNT(*) FILTER (WHERE status = 1) AS unstable_count,
			COUNT(*) FILTER (WHERE status = 0) AS offline_count,
			COUNT(latency_ms) AS latency_count,
			AVG(latency_ms)::float8 AS avg_latency_ms,
			MIN(latency_ms)::float8 AS min_latency_ms,
			MAX(latency_ms)::float8 AS max_latency_ms`).
		Where("tenant_id = ? AND time >= ? AND time < ?", tenantId, start, end)
	if ipId > 0 {
		query = query.Where("ip_id = ?", ipId)
	}
	if tacticsId > 0 {
		query = query.Where("tactics_id = ?", tacticsId)
	}
	err := query.Group("bucket").Order("bucket").Scan(&rows).Error
	return rows, err
}

// ListByTimeRange 查询 [start, end) 范围内的 ping 探测明细记录，按时间升序。
// ipId / tacticsId 传 0 表示不按该条件过滤。
func (r *PingResultRepository) ListByTimeRange(tenantId string, start, end time.Time, ipId, tacticsId uint64) ([]model.PingResult, error) {
	results := make([]model.PingResult, 0, 64)
	query := db.Where("tenant_id = ? AND time >= ? AND time < ?", tenantId, start, end)
	if ipId > 0 {
		query = query.Where("ip_id = ?", ipId)
	}
	if tacticsId > 0 {
		query = query.Where("tactics_id = ?", tacticsId)
	}
	err := query.Order("time").Find(&results).Error
	return results, err
}
