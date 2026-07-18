package dto

import (
	"time"

	"bs-net-monitor/internal/model"
)

// IPCreateRequest 创建 IP 的请求参数。
type IPCreateRequest struct {
	Name      string `json:"name" binding:"required"`
	Ip        string `json:"ip" binding:"required"`
	Position  string `json:"position"`
	Remark    string `json:"remark"`
	TacticsId uint64 `json:"tacticsId"`
	Enabled   bool   `json:"enabled"`
}

// IPUpdateRequest 更新 IP 的请求参数。
type IPUpdateRequest struct {
	Name      string `json:"name"`
	Ip        string `json:"ip"`
	Position  string `json:"position"`
	Remark    string `json:"remark"`
	TacticsId uint64 `json:"tacticsId"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

// IPListQuery IP 列表查询参数。
type IPListQuery struct {
	Current   int    `form:"current,default=1"`
	Size      int    `form:"size,default=15"`
	TacticsId uint64 `form:"tacticsId"`
	Enabled   *bool  `form:"enabled"`
}

// PageRes 分页响应结构。
type PageRes[T any] struct {
	Total   int64 `json:"total"`
	Records []T   `json:"records"`
	Current int   `json:"current"`
	Size    int   `json:"size"`
	Pages   int   `json:"pages"`
}

// IPBatchUpdateRequest 批量更新 IP 启用状态的请求参数。
type IPBatchUpdateRequest struct {
	IpIds   []uint64 `json:"ipIds" binding:"required"`
	Enabled bool     `json:"enabled"`
}

// IPBatchDeleteRequest 批量删除 IP 的请求参数。
type IPBatchDeleteRequest struct {
	IpIds []uint64 `json:"ipIds" binding:"required"`
}

// IPResponse IP 接口的响应数据。
type IPResponse struct {
	IpId        uint64    `json:"ipId"`
	TenantId    string    `json:"tenantId"`
	Name        string    `json:"name"`
	Ip          string    `json:"ip"`
	Position    string    `json:"position"`
	Remark      string    `json:"remark"`
	TacticsId   uint64    `json:"tacticsId"`
	TacticsName string    `json:"tacticsName"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ToIPResponse 将 model.IP 转换为 IPResponse。
func ToIPResponse(ip *model.IP, tacticsName string) *IPResponse {
	if ip == nil {
		return nil
	}
	return &IPResponse{
		IpId:        ip.IpId,
		TenantId:    ip.TenantId,
		Name:        ip.Name,
		Ip:          ip.Ip,
		Position:    ip.Position,
		Remark:      ip.Remark,
		TacticsId:   ip.TacticsId,
		TacticsName: tacticsName,
		Enabled:     ip.Enabled,
		CreatedAt:   ip.CreatedAt.Local(),
		UpdatedAt:   ip.UpdatedAt.Local(),
	}
}

// ToIPResponseList 将 []model.IP 转换为 []IPResponse。
func ToIPResponseList(ips []model.IP, tacticsMap map[uint64]string) []IPResponse {
	list := make([]IPResponse, 0, len(ips))
	for i := range ips {
		name := tacticsMap[ips[i].TacticsId]
		list = append(list, *ToIPResponse(&ips[i], name))
	}
	return list
}
