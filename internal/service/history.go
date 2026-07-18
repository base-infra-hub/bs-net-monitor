package service

import (
	"errors"
	"math"
	"sync"
	"time"

	"bs-net-monitor/internal/detector"
	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/model"
	"bs-net-monitor/internal/repository"
)

// ErrInvalidDate 历史查询的日期参数格式不正确。
var ErrInvalidDate = errors.New("日期格式无效，正确格式：2006-01-02")

// ErrInvalidRange 历史查询的日期范围不正确。
var ErrInvalidRange = errors.New("日期范围无效：结束日期不能早于开始日期，且跨度不能超过 7 天")

// ErrInvalidTime 分钟明细查询的 time 参数格式不正确。
var ErrInvalidTime = errors.New("time 格式无效，支持格式：2006-01-02 15:04:05 或 RFC3339（如 2026-07-18T12:00:00+08:00）")

// maxRangeDays 历史查询允许的最大自然日数。
const maxRangeDays = 7

// windowsPerDay 一天的分钟窗口数。
const windowsPerDay = 24 * 60

// HistoryService 处理历史数据聚合查询。
type HistoryService struct {
	repo        *repository.PingResultRepository
	ipRepo      *repository.IPRepository
	tacticsRepo *repository.TacticsRepository
}

var (
	historyServiceInstance *HistoryService
	historyServiceOnce     sync.Once
)

// GetHistoryService 返回历史查询服务的单例。
func GetHistoryService() *HistoryService {
	historyServiceOnce.Do(func() {
		historyServiceInstance = &HistoryService{
			repo:        repository.GetPingResultRepository(),
			ipRepo:      repository.GetIPRepository(),
			tacticsRepo: repository.GetTacticsRepository(),
		}
	})
	return historyServiceInstance
}

// QueryRange 查询日期范围内（服务器本地时区，最大 7 天）的连接状态，按一分钟窗口聚合，
// 可选按策略组或单个 IP 过滤。
func (s *HistoryService) QueryRange(tenantId string, query dto.HistoryQuery) (*dto.HistoryResponse, error) {
	rangeStart, err := time.ParseInLocation("2006-01-02", query.StartDate, time.Local)
	if err != nil {
		return nil, ErrInvalidDate
	}
	endDate := query.EndDate
	if endDate == "" {
		endDate = query.StartDate
	}
	rangeEndDay, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		return nil, ErrInvalidDate
	}
	if rangeEndDay.Before(rangeStart) || rangeEndDay.Sub(rangeStart) >= maxRangeDays*24*time.Hour {
		return nil, ErrInvalidRange
	}
	days := int(rangeEndDay.Sub(rangeStart).Hours()/24) + 1
	rangeEnd := rangeEndDay.AddDate(0, 0, 1)

	rows, err := s.repo.AggregateByMinute(tenantId, rangeStart.UTC(), rangeEnd.UTC(), query.IpId, query.TacticsId)
	if err != nil {
		return nil, err
	}

	// 各策略组的不稳定阈值，用于判定窗口平均延迟的状态
	tacticsList, err := s.tacticsRepo.List(tenantId)
	if err != nil {
		return nil, err
	}
	unstableMsMap := make(map[uint64]int, len(tacticsList))
	for _, t := range tacticsList {
		unstableMsMap[t.TacticsId] = t.UnstableMs
	}

	rowMap := make(map[int64]repository.MinuteAggRow, len(rows))
	for _, row := range rows {
		rowMap[row.Bucket.UTC().Truncate(time.Minute).Unix()] = row
	}

	totalWindows := days * windowsPerDay
	windows := make([]dto.HistoryWindow, 0, totalWindows)
	summary := dto.HistorySummary{}
	var latencySum float64
	var latencyTotal int

	for i := 0; i < totalWindows; i++ {
		winStart := rangeStart.Add(time.Duration(i) * time.Minute)
		w := dto.HistoryWindow{
			Time:          winStart,
			Status:        dto.HistoryStatusNoData,
			LatencyStatus: dto.HistoryStatusNoData,
		}
		row, ok := rowMap[winStart.UTC().Unix()]
		if !ok {
			windows = append(windows, w)
			continue
		}

		w.OnlineCount = row.OnlineCount
		w.UnstableCount = row.UnstableCount
		w.OfflineCount = row.OfflineCount
		w.Total = row.OnlineCount + row.UnstableCount + row.OfflineCount
		w.AvgLatencyMs = roundLatency(row.AvgLatencyMs)
		w.MinLatencyMs = roundLatency(row.MinLatencyMs)
		w.MaxLatencyMs = roundLatency(row.MaxLatencyMs)
		w.Status = dominantStatus(row)
		w.LatencyStatus = latencyStatus(row, unstableMsMap)
		windows = append(windows, w)

		summary.OnlineCount += row.OnlineCount
		summary.UnstableCount += row.UnstableCount
		summary.OfflineCount += row.OfflineCount
		if row.AvgLatencyMs != nil && row.LatencyCount > 0 {
			latencySum += *row.AvgLatencyMs * float64(row.LatencyCount)
			latencyTotal += row.LatencyCount
		}
	}

	summary.Total = summary.OnlineCount + summary.UnstableCount + summary.OfflineCount
	if summary.Total > 0 {
		summary.OnlineRate = math.Round(float64(summary.OnlineCount)/float64(summary.Total)*1000) / 10
	}
	if latencyTotal > 0 {
		avg := math.Round(latencySum/float64(latencyTotal)*10) / 10
		summary.AvgLatencyMs = &avg
	}

	return &dto.HistoryResponse{
		StartDate: query.StartDate,
		EndDate:   endDate,
		Summary:   summary,
		Windows:   windows,
	}, nil
}

// dominantStatus 返回窗口内出现次数最多的状态；并列时取较差的状态（断线 > 不稳定 > 稳定），
// 以更保守地反映该分钟的连接质量。
func dominantStatus(row repository.MinuteAggRow) int {
	status, max := detector.StatusOffline, row.OfflineCount
	if row.UnstableCount > max {
		status, max = detector.StatusUnstable, row.UnstableCount
	}
	if row.OnlineCount > max {
		status = detector.StatusOnline
	}
	return status
}

// latencyStatus 返回窗口平均延迟对应的状态：把窗口平均延迟（超时记录不参与计算）与
// 窗口主导策略组的 unstableMs 阈值比较，判定逻辑与探测写入时一致
// （unstableMs > 0 且延迟超过阈值才算不稳定）。窗口全部断联（无任何延迟值）时返回断线，
// 表示该分钟整体超时；策略组已被删除时退化为按记录状态计数判定。
func latencyStatus(row repository.MinuteAggRow, unstableMsMap map[uint64]int) int {
	if row.LatencyCount == 0 {
		return detector.StatusOffline
	}
	threshold, ok := unstableMsMap[row.DominantTacticsId]
	if !ok {
		if row.UnstableCount > row.OnlineCount {
			return detector.StatusUnstable
		}
		return detector.StatusOnline
	}
	if threshold > 0 && row.AvgLatencyMs != nil && *row.AvgLatencyMs > float64(threshold) {
		return detector.StatusUnstable
	}
	return detector.StatusOnline
}

// QueryMinuteDetail 查询某一分钟窗口内的全部探测明细记录，可选按策略组或单个 IP 过滤。
func (s *HistoryService) QueryMinuteDetail(tenantId string, query dto.HistoryDetailQuery) ([]dto.HistoryRecord, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", query.Time, time.Local)
	if err != nil {
		// 兼容 RFC3339 带时区格式（前端可能直接回传窗口时间）
		t, err = time.Parse(time.RFC3339, query.Time)
		if err != nil {
			return nil, ErrInvalidTime
		}
	}
	start := t.Truncate(time.Minute)
	end := start.Add(time.Minute)

	results, err := s.repo.ListByTimeRange(tenantId, start.UTC(), end.UTC(), query.IpId, query.TacticsId)
	if err != nil {
		return nil, err
	}

	// 补充 IP 的名称和地址
	ipIds := make([]uint64, 0, len(results))
	seen := make(map[uint64]bool, len(results))
	for _, r := range results {
		if !seen[r.IpId] {
			seen[r.IpId] = true
			ipIds = append(ipIds, r.IpId)
		}
	}
	ipMap := make(map[uint64]model.IP, len(ipIds))
	if len(ipIds) > 0 {
		ips, err := s.ipRepo.ListByIDs(tenantId, ipIds)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			ipMap[ip.IpId] = ip
		}
	}

	records := make([]dto.HistoryRecord, 0, len(results))
	for _, r := range results {
		ip := ipMap[r.IpId]
		records = append(records, dto.HistoryRecord{
			Time:      r.Time.Local(),
			IpId:      r.IpId,
			Name:      ip.Name,
			Ip:        ip.Ip,
			LatencyMs: r.LatencyMs,
			Status:    r.Status,
		})
	}
	return records, nil
}

// roundLatency 将平均延迟保留一位小数。
func roundLatency(avg *float64) *float64 {
	if avg == nil {
		return nil
	}
	v := math.Round(*avg*10) / 10
	return &v
}
