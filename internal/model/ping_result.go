package model

import "time"

// PingResult 保存单次 ping 探测的结果，设计为按时间分区的 TimescaleDB 超表。
// 复合主键 (time, ip_id) 避免多 IP 在同一时刻探测时主键冲突。
type PingResult struct {
	Time      time.Time `json:"time" gorm:"primaryKey;column:time"`
	TenantId  string    `json:"tenantId" gorm:"index;not null;column:tenant_id"`
	IpId      uint64    `json:"ipId" gorm:"primaryKey;index;not null;column:ip_id"`
	TacticsId uint64    `json:"tacticsId" gorm:"index;not null;column:tactics_id"`
	LatencyMs *int      `json:"latencyMs" gorm:"column:latency_ms"`
	Status    int       `json:"status" gorm:"not null;column:status"`
}
