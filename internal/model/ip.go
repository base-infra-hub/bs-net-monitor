package model

import "time"

// IP 表示一个被监控的 IP 地址，属于某个租户和某个策略组。
type IP struct {
	IpId      uint64    `json:"ipId" gorm:"primaryKey;column:ip_id"`
	TenantId  string    `json:"tenantId" gorm:"index;not null;column:tenant_id"`
	Name      string    `json:"name" gorm:"not null"`
	Ip        string    `json:"ip" gorm:"not null;column:ip"`
	Position  string    `json:"position" gorm:"column:position"`
	Remark    string    `json:"remark"`
	TacticsId uint64    `json:"tacticsId" gorm:"index;column:tactics_id"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
