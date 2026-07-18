package model

import "time"

// Tactics 表示一个检测策略组。
type Tactics struct {
	TacticsId  uint64    `json:"tacticsId" gorm:"primaryKey;column:tactics_id"`
	TenantId   string    `json:"tenantId" gorm:"index;not null;column:tenant_id"`
	Name       string    `json:"name" gorm:"not null"`
	IntervalMs int       `json:"intervalMs" gorm:"default:60000;column:interval_ms"`
	TimeoutMs  int       `json:"timeoutMs" gorm:"default:3000;column:timeout_ms"`
	UnstableMs int       `json:"unstableMs" gorm:"default:0;column:unstable_ms"`
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
