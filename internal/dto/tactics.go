package dto

import (
	"time"

	"bs-net-monitor/internal/model"
)

// TacticsCreateRequest 创建策略组的请求参数。
type TacticsCreateRequest struct {
	Name       string `json:"name" binding:"required"`
	IntervalMs int    `json:"intervalMs"`
	TimeoutMs  int    `json:"timeoutMs"`
	UnstableMs int    `json:"unstableMs"`
	Enabled    bool   `json:"enabled"`
}

// TacticsUpdateRequest 更新策略组的请求参数。
type TacticsUpdateRequest struct {
	Name       string `json:"name"`
	IntervalMs int    `json:"intervalMs"`
	TimeoutMs  int    `json:"timeoutMs"`
	UnstableMs int    `json:"unstableMs"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// TacticsResponse 策略组接口的响应数据。
type TacticsResponse struct {
	TacticsId  uint64    `json:"tacticsId"`
	TenantId   string    `json:"tenantId"`
	Name       string    `json:"name"`
	IntervalMs int       `json:"intervalMs"`
	TimeoutMs  int       `json:"timeoutMs"`
	UnstableMs int       `json:"unstableMs"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// ToTacticsResponse 将 model.Tactics 转换为 TacticsResponse。
func ToTacticsResponse(t *model.Tactics) *TacticsResponse {
	if t == nil {
		return nil
	}
	return &TacticsResponse{
		TacticsId:  t.TacticsId,
		TenantId:   t.TenantId,
		Name:       t.Name,
		IntervalMs: t.IntervalMs,
		TimeoutMs:  t.TimeoutMs,
		UnstableMs: t.UnstableMs,
		Enabled:    t.Enabled,
		CreatedAt:  t.CreatedAt.Local(),
		UpdatedAt:  t.UpdatedAt.Local(),
	}
}

// ToTacticsResponseList 将 []model.Tactics 转换为 []TacticsResponse。
func ToTacticsResponseList(list []model.Tactics) []TacticsResponse {
	resp := make([]TacticsResponse, 0, len(list))
	for i := range list {
		resp = append(resp, *ToTacticsResponse(&list[i]))
	}
	return resp
}
