package dto

import (
	"time"
)

// HistoryStatusNoData 表示该时间窗口内没有任何探测数据。
const HistoryStatusNoData = -1

// HistoryQuery 历史聚合查询参数：startDate 为必填（格式 2006-01-02，按服务器本地时区解释），
// endDate 可选（缺省等于 startDate，即查单日），最大跨度 7 天。
// tacticsId / ipId 为可选过滤条件，可同时使用。
type HistoryQuery struct {
	StartDate string `form:"startDate" binding:"required"`
	EndDate   string `form:"endDate"`
	TacticsId uint64 `form:"tacticsId"`
	IpId      uint64 `form:"ipId"`
}

// HistoryWindow 一分钟聚合窗口的连接状态统计。
type HistoryWindow struct {
	Time          time.Time `json:"time"`          // 窗口开始时间（RFC3339 带时区）
	Status        int       `json:"status"`        // 窗口主导状态：0 断线 1 不稳定 2 稳定 -1 无数据
	LatencyStatus int       `json:"latencyStatus"` // 窗口平均延迟按主导策略 unstableMs 阈值判定的状态：0 超时（全部断联）1 不稳定 2 稳定 -1 无数据
	OnlineCount   int       `json:"onlineCount"`   // 稳定次数
	UnstableCount int       `json:"unstableCount"` // 不稳定次数
	OfflineCount  int       `json:"offlineCount"`  // 断线次数
	Total         int       `json:"total"`         // 窗口内探测总次数
	AvgLatencyMs  *float64  `json:"avgLatencyMs"`  // 平均延迟（断线记录不计入），无延迟数据时为 null
	MinLatencyMs  *float64  `json:"minLatencyMs"`  // 该分钟内最小延迟，无数据时为 null
	MaxLatencyMs  *float64  `json:"maxLatencyMs"`  // 该分钟内最大延迟，无数据时为 null
}

// HistorySummary 当天整体的汇总统计。
type HistorySummary struct {
	OnlineCount   int      `json:"onlineCount"`
	UnstableCount int      `json:"unstableCount"`
	OfflineCount  int      `json:"offlineCount"`
	Total         int      `json:"total"`
	OnlineRate    float64  `json:"onlineRate"`   // 稳定率（百分比，0-100）
	AvgLatencyMs  *float64 `json:"avgLatencyMs"` // 当天平均延迟，无延迟数据时为 null
}

// HistoryResponse 历史聚合查询的响应数据。
type HistoryResponse struct {
	StartDate string          `json:"startDate"`
	EndDate   string          `json:"endDate"`
	Summary   HistorySummary  `json:"summary"`
	Windows   []HistoryWindow `json:"windows"`
}

// HistoryDetailQuery 分钟明细查询参数：time 为必填（支持 2006-01-02 15:04:05 或 RFC3339 带时区格式），
// 按分钟取整，tacticsId / ipId 为可选过滤条件。
type HistoryDetailQuery struct {
	Time      string `form:"time" binding:"required"`
	TacticsId uint64 `form:"tacticsId"`
	IpId      uint64 `form:"ipId"`
}

// HistoryRecord 一分钟窗口内的单条探测记录明细。
type HistoryRecord struct {
	Time      time.Time `json:"time"` // 探测时间（RFC3339 带时区）
	IpId      uint64    `json:"ipId"`
	Name      string    `json:"name"`      // IP 名称
	Ip        string    `json:"ip"`        // IP 地址
	LatencyMs *int      `json:"latencyMs"` // 延迟，断线时为 null
	Status    int       `json:"status"`    // 0 断线 1 不稳定 2 稳定
}
