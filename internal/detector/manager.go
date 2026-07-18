package detector

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/model"
	"bs-net-monitor/internal/repository"
)

// Manager 是检测引擎的单例入口。
// 所有状态变更通过事件循环串行处理；probes 只在事件循环 goroutine 访问。
// tactics 由 tacticsMu 保护；latestResults/tacticsIPs/tenantIPs 由 latestMu 保护。
type Manager struct {
	probes        map[uint64]*probeHandle  // 以 ip_id 为键，仅在事件循环 goroutine 中访问
	tacticsMu     sync.RWMutex             // 保护 tactics
	tactics       map[uint64]model.Tactics // 以 tactics_id 为键
	tacticsIPs    map[uint64][]uint64      // tactics_id -> ip_id 数组，用于停止探测及实时查询
	tenantIPs     map[string][]uint64      // tenant_id -> ip_id 数组，用于租户级实时统计
	pingRepo      *repository.PingResultRepository
	latestMu      sync.RWMutex                // 保护 latestResults/tacticsIPs/tenantIPs
	latestResults map[uint64]dto.PingStatusVO // 以 ip_id 为键，最新 ping 状态
	managerStop   chan struct{}
	events        chan Event
	wg            sync.WaitGroup
	loopWg        sync.WaitGroup
	started       atomic.Bool
	stopOnce      sync.Once
}

type probeHandle struct {
	cancel context.CancelFunc
	info   probeInfo
}

// probeInfo 只保留探测一个 IP 所需的最小信息。
type probeInfo struct {
	ipId      uint64
	tenantId  string
	ip        string
	tacticsId uint64
}

// Event 是提交给检测引擎的事件接口。
// 服务层通过 New* 构造函数创建具体事件，再调用 Manager.Submit 提交。
type Event interface{ tag() }

type (
	evIPBatchChanged struct{ IPs []model.IP }
	evIPBatchDeleted struct{ IpIds []uint64 }
	evTacticsCreated struct{ Tactics model.Tactics }
	evTacticsUpdated struct{ Tactics model.Tactics }
	evTacticsDeleted struct{ TacticsId uint64 }
)

func (evIPBatchChanged) tag() {}
func (evIPBatchDeleted) tag() {}
func (evTacticsCreated) tag() {}
func (evTacticsUpdated) tag() {}
func (evTacticsDeleted) tag() {}

// NewIPBatchChangedEvent 创建 IP 变更事件（创建/更新/启用/停用）。
func NewIPBatchChangedEvent(ips []model.IP) Event { return evIPBatchChanged{IPs: ips} }

// NewIPBatchDeletedEvent 创建 IP 删除事件，支持单条或多条。
func NewIPBatchDeletedEvent(ipIds []uint64) Event { return evIPBatchDeleted{IpIds: ipIds} }

// NewTacticsCreatedEvent 创建策略组创建事件。
func NewTacticsCreatedEvent(t model.Tactics) Event { return evTacticsCreated{Tactics: t} }

// NewTacticsUpdatedEvent 创建策略组更新事件。
func NewTacticsUpdatedEvent(t model.Tactics) Event { return evTacticsUpdated{Tactics: t} }

// NewTacticsDeletedEvent 创建策略组删除事件。
func NewTacticsDeletedEvent(tacticsId uint64) Event { return evTacticsDeleted{TacticsId: tacticsId} }

var (
	managerInstance *Manager
	managerOnce     sync.Once
)

// GetManager 返回检测引擎管理器的单例。
func GetManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
	})
	return managerInstance
}

// RegisterShutdownHook 在收到 SIGINT/SIGTERM 时优雅停止检测引擎。
// 应在 main 中调用，保证进程退出前释放资源。
func (m *Manager) RegisterShutdownHook() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[detector] 收到退出信号，开始优雅停机...")
		m.Stop()
	}()
}

// Start 初始化并启动检测引擎。
// 该方法只在应用启动时调用一次，调用方唯一，因此初始化阶段不需要互斥锁。
func (m *Manager) Start() error {
	if m.started.Load() {
		return nil
	}

	m.pingRepo = repository.GetPingResultRepository()
	m.probes = make(map[uint64]*probeHandle)
	m.tactics = make(map[uint64]model.Tactics)
	m.tacticsIPs = make(map[uint64][]uint64)
	m.tenantIPs = make(map[string][]uint64)
	m.latestResults = make(map[uint64]dto.PingStatusVO)
	m.managerStop = make(chan struct{})
	m.events = make(chan Event, 64)

	// 先全量同步：事件循环尚未启动，probes/tactics/tacticsIPs/tenantIPs 只在当前 goroutine 访问，避免竞态。
	m.fullSync()

	m.loopWg.Add(1)
	go m.runEventLoop()

	m.started.Store(true)
	return nil
}

// Stop 优雅地停止检测引擎。可安全重复调用。
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		if !m.started.Load() {
			return
		}
		close(m.managerStop)
		m.loopWg.Wait()
	})
}

// Submit 向检测引擎提交一个事件。事件将在内部事件循环中串行处理。
func (m *Manager) Submit(e Event) {
	m.events <- e
}

// runEventLoop 是事件循环，串行处理所有状态变更。
// probes 只在该 goroutine 中访问；tacticsIPs/tenantIPs/latestResults 的写也在该 goroutine 中，
// 但读可能并发，因此写操作需加 latestMu。
func (m *Manager) runEventLoop() {
	defer m.loopWg.Done()

	for {
		select {
		case e := <-m.events:
			switch ev := e.(type) {
			case evTacticsCreated:
				m.handleTacticsCreated(ev.Tactics)
			case evTacticsUpdated:
				m.handleTacticsUpdated(ev.Tactics)
			case evTacticsDeleted:
				m.handleTacticsDeleted(ev.TacticsId)
			case evIPBatchChanged:
				for i := range ev.IPs {
					m.ensureProbe(ev.IPs[i])
				}
			case evIPBatchDeleted:
				for _, ipId := range ev.IpIds {
					m.removeProbe(ipId)
				}
			}
		case <-m.managerStop:
			m.handleStop()
			return
		}
	}
}

func (m *Manager) handleTacticsCreated(t model.Tactics) {
	if !t.Enabled {
		return
	}
	m.updateTactics(t)
}

func (m *Manager) handleTacticsUpdated(t model.Tactics) {
	m.tacticsMu.RLock()
	old, existed := m.tactics[t.TacticsId]
	m.tacticsMu.RUnlock()

	if !t.Enabled {
		if existed {
			m.stopByTacticsId(t.TacticsId)
			m.deleteTactics(t.TacticsId)
		}
		return
	}

	// 策略从禁用变为启用，需要启动其下所有已启用 IP。
	if existed && !old.Enabled {
		m.startIPsByTactics(t.TenantId, t.TacticsId)
	}

	m.updateTactics(t)
}

func (m *Manager) handleTacticsDeleted(tacticsId uint64) {
	m.stopByTacticsId(tacticsId)
	m.deleteTactics(tacticsId)
}

func (m *Manager) handleStop() {
	for _, h := range m.probes {
		h.cancel()
	}
	m.probes = make(map[uint64]*probeHandle)
	m.tactics = make(map[uint64]model.Tactics)

	m.latestMu.Lock()
	m.tacticsIPs = make(map[uint64][]uint64)
	m.tenantIPs = make(map[string][]uint64)
	m.latestResults = make(map[uint64]dto.PingStatusVO)
	m.latestMu.Unlock()

	m.wg.Wait()
	m.started.Store(false)
	log.Println("[detector] 检测引擎已停止")
}

// LatestStatus 返回指定 IP 的最新 ping 状态。
func (m *Manager) LatestStatus(ipId uint64) (dto.PingStatusVO, bool) {
	m.latestMu.RLock()
	defer m.latestMu.RUnlock()
	v, ok := m.latestResults[ipId]
	return v, ok
}

// LatestStatuses 返回指定租户下所有 IP 的最新状态。
func (m *Manager) LatestStatuses(tenantId string) []dto.PingStatusVO {
	m.latestMu.RLock()
	defer m.latestMu.RUnlock()

	ipIds := m.tenantIPs[tenantId]
	list := make([]dto.PingStatusVO, 0, len(ipIds))
	for _, ipId := range ipIds {
		if v, ok := m.latestResults[ipId]; ok {
			list = append(list, v)
		}
	}
	return list
}

// LatestStatusesByTactics 返回指定租户、指定策略组下所有 IP 的最新状态。
func (m *Manager) LatestStatusesByTactics(tenantId string, tacticsId uint64) []dto.PingStatusVO {
	m.latestMu.RLock()
	defer m.latestMu.RUnlock()

	ipIds := m.tacticsIPs[tacticsId]
	list := make([]dto.PingStatusVO, 0, len(ipIds))
	for _, ipId := range ipIds {
		if v, ok := m.latestResults[ipId]; ok && v.TenantId == tenantId {
			list = append(list, v)
		}
	}
	return list
}

// Statistic 返回指定租户下的在线/离线/不稳定数量。
func (m *Manager) Statistic(tenantId string) (online, offline, unstable int) {
	m.latestMu.RLock()
	defer m.latestMu.RUnlock()

	for _, ipId := range m.tenantIPs[tenantId] {
		v, ok := m.latestResults[ipId]
		if !ok {
			continue
		}
		switch v.Status {
		case StatusOnline:
			online++
		case StatusOffline:
			offline++
		case StatusUnstable:
			unstable++
		}
	}
	return
}

// ResultCh 返回实时结果发布通道，供 live handler 订阅。
// fullSync 全量同步，仅在启动时使用。
func (m *Manager) fullSync() {
	tacticsMap, err := m.loadTactics()
	if err != nil {
		log.Printf("[detector] 启动时加载策略组失败: %v", err)
		return
	}
	m.tactics = tacticsMap

	infos, err := m.loadProbeInfos(tacticsMap)
	if err != nil {
		log.Printf("[detector] 启动时加载 IP 失败: %v", err)
		return
	}

	for _, info := range infos {
		m.addProbe(info)
	}
}

func (m *Manager) loadTactics() (map[uint64]model.Tactics, error) {
	list, err := repository.GetTacticsRepository().ListAllEnabled()
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]model.Tactics, len(list))
	for i := range list {
		result[list[i].TacticsId] = list[i]
	}
	return result, nil
}

func (m *Manager) loadProbeInfos(tacticsMap map[uint64]model.Tactics) (map[uint64]probeInfo, error) {
	ips, err := repository.GetIPRepository().ListAllEnabled()
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]probeInfo, len(ips))
	for i := range ips {
		if _, ok := tacticsMap[ips[i].TacticsId]; !ok {
			continue
		}
		result[ips[i].IpId] = probeInfo{
			ipId:      ips[i].IpId,
			tenantId:  ips[i].TenantId,
			ip:        ips[i].Ip,
			tacticsId: ips[i].TacticsId,
		}
	}
	return result, nil
}

func (m *Manager) updateTactics(t model.Tactics) {
	m.tacticsMu.Lock()
	m.tactics[t.TacticsId] = t
	m.tacticsMu.Unlock()
}

func (m *Manager) deleteTactics(tacticsId uint64) {
	m.tacticsMu.Lock()
	delete(m.tactics, tacticsId)
	m.tacticsMu.Unlock()
}

// ensureProbe 根据 IP 当前状态决定启动、重启或停止探测。
func (m *Manager) ensureProbe(ip model.IP) {
	info := probeInfo{
		ipId:      ip.IpId,
		tenantId:  ip.TenantId,
		ip:        ip.Ip,
		tacticsId: ip.TacticsId,
	}

	m.tacticsMu.RLock()
	t, tacticOK := m.tactics[ip.TacticsId]
	m.tacticsMu.RUnlock()
	shouldRun := ip.Enabled && tacticOK && t.Enabled

	old, running := m.probes[ip.IpId]

	if !shouldRun {
		if running {
			m.removeProbe(ip.IpId)
		}
		return
	}

	if running && old.info == info {
		return
	}

	if running {
		m.removeProbe(ip.IpId)
	}
	m.addProbe(info)
}

// startIPsByTactics 启动某个策略组下所有已启用的 IP。
func (m *Manager) startIPsByTactics(tenantId string, tacticsId uint64) {
	ips, err := repository.GetIPRepository().ListByTactics(tenantId, tacticsId)
	if err != nil {
		log.Printf("[detector] 获取策略组 %d 下的 IP 失败: %v", tacticsId, err)
		return
	}
	for i := range ips {
		if !ips[i].Enabled {
			continue
		}
		m.ensureProbe(ips[i])
	}
}

// stopByTacticsId 停止某个策略组下的所有探测协程。
func (m *Manager) stopByTacticsId(tacticsId uint64) {
	ipIds := m.tacticsIPs[tacticsId]
	if len(ipIds) == 0 {
		return
	}
	// 复制一份再遍历，避免 removeProbe 修改 tacticsIPs 切片时影响循环。
	copyIds := make([]uint64, len(ipIds))
	copy(copyIds, ipIds)
	for _, ipId := range copyIds {
		m.removeProbe(ipId)
	}
}

func (m *Manager) addProbe(info probeInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	m.probes[info.ipId] = &probeHandle{cancel: cancel, info: info}

	m.latestMu.Lock()
	m.tacticsIPs[info.tacticsId] = appendUniqueUint64(m.tacticsIPs[info.tacticsId], info.ipId)
	m.tenantIPs[info.tenantId] = appendUniqueUint64(m.tenantIPs[info.tenantId], info.ipId)
	m.latestMu.Unlock()

	m.wg.Add(1)
	go m.runProbe(ctx, info)
}

func (m *Manager) removeProbe(ipId uint64) {
	h, ok := m.probes[ipId]
	if !ok {
		return
	}
	h.cancel()
	delete(m.probes, ipId)

	m.latestMu.Lock()
	if list, ok := m.tacticsIPs[h.info.tacticsId]; ok {
		m.tacticsIPs[h.info.tacticsId] = removeUint64(list, ipId)
		if len(m.tacticsIPs[h.info.tacticsId]) == 0 {
			delete(m.tacticsIPs, h.info.tacticsId)
		}
	}
	if list, ok := m.tenantIPs[h.info.tenantId]; ok {
		m.tenantIPs[h.info.tenantId] = removeUint64(list, ipId)
		if len(m.tenantIPs[h.info.tenantId]) == 0 {
			delete(m.tenantIPs, h.info.tenantId)
		}
	}
	delete(m.latestResults, ipId)
	m.latestMu.Unlock()
}

func (m *Manager) runProbe(ctx context.Context, info probeInfo) {
	defer m.wg.Done()

	// 启动时立即执行一次探测，避免等待第一个周期（例如5~10秒）才首次显示数据
	m.probeOnce(info)

	ticker := time.NewTicker(m.intervalFor(info.tacticsId))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeOnce(info)
			ticker.Reset(m.intervalFor(info.tacticsId))
		}
	}
}

func (m *Manager) intervalFor(tacticsId uint64) time.Duration {
	m.tacticsMu.RLock()
	t, ok := m.tactics[tacticsId]
	m.tacticsMu.RUnlock()
	if !ok || t.IntervalMs <= 0 {
		return 60 * time.Second
	}
	return time.Duration(t.IntervalMs) * time.Millisecond
}

func (m *Manager) probeOnce(info probeInfo) {
	m.tacticsMu.RLock()
	t, ok := m.tactics[info.tacticsId]
	m.tacticsMu.RUnlock()
	if !ok {
		return
	}

	res := runProbe(&ProbeTask{
		Ip:         info.ip,
		TimeoutMs:  t.TimeoutMs,
		UnstableMs: t.UnstableMs,
	})

	result := model.PingResult{
		Time:      res.Time.UTC(),
		TenantId:  info.tenantId,
		IpId:      info.ipId,
		TacticsId: info.tacticsId,
		LatencyMs: res.LatencyMs,
		Status:    res.Status,
	}

	if err := m.pingRepo.Create(&result); err != nil {
		log.Printf("[detector] 写入 IP %d 的 ping 结果失败: %v", info.ipId, err)
	}

	vo := dto.ToPingStatusVO(result)
	m.latestMu.Lock()
	m.latestResults[info.ipId] = vo
	m.latestMu.Unlock()
}

func appendUniqueUint64(list []uint64, v uint64) []uint64 {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func removeUint64(list []uint64, v uint64) []uint64 {
	for i, x := range list {
		if x == v {
			list[i] = list[len(list)-1]
			return list[:len(list)-1]
		}
	}
	return list
}
