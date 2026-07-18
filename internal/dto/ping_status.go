package dto

import (
	"time"

	"bs-net-monitor/internal/model"
)

// PingStatusVO 表示单个 IP 的最新 ping 状态，是项目中最小颗粒度的实时结果 VO。
type PingStatusVO struct {
	IpId      uint64    `json:"ipId"`
	TenantId  string    `json:"-"`
	TacticsId uint64    `json:"tacticsId"`
	LatencyMs *int      `json:"latencyMs"`
	Status    int       `json:"status"`
	Time      time.Time `json:"time"`
}

// ToPingStatusVO 将 model.PingResult 转换为 PingStatusVO。
func ToPingStatusVO(r model.PingResult) PingStatusVO {
	return PingStatusVO{
		IpId:      r.IpId,
		TenantId:  r.TenantId,
		TacticsId: r.TacticsId,
		LatencyMs: r.LatencyMs,
		Status:    r.Status,
		Time:      r.Time.Local(),
	}
}
