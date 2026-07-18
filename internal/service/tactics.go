package service

import (
	"errors"
	"sync"

	"bs-net-monitor/internal/detector"
	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/model"
	"bs-net-monitor/internal/repository"
)

// TacticsService 处理策略组相关的业务逻辑。
type TacticsService struct {
	repo   *repository.TacticsRepository
	ipRepo *repository.IPRepository
}

var (
	tacticsServiceInstance *TacticsService
	tacticsServiceOnce     sync.Once
)

// GetTacticsService 返回策略组服务的单例。
func GetTacticsService() *TacticsService {
	tacticsServiceOnce.Do(func() {
		tacticsServiceInstance = &TacticsService{
			repo:   repository.GetTacticsRepository(),
			ipRepo: repository.GetIPRepository(),
		}
	})
	return tacticsServiceInstance
}

func (s *TacticsService) List(tenantId string) ([]dto.TacticsResponse, error) {
	list, err := s.repo.List(tenantId)
	if err != nil {
		return nil, err
	}
	return dto.ToTacticsResponseList(list), nil
}

func (s *TacticsService) Create(tenantId string, req *dto.TacticsCreateRequest) (*dto.TacticsResponse, error) {
	interval := req.IntervalMs
	if interval <= 0 {
		interval = 60000
	}
	timeout := req.TimeoutMs
	if timeout <= 0 {
		timeout = 3000
	}

	tactics := &model.Tactics{
		TenantId:   tenantId,
		Name:       req.Name,
		IntervalMs: interval,
		TimeoutMs:  timeout,
		UnstableMs: req.UnstableMs,
		Enabled:    req.Enabled,
	}

	if err := validateTactics(tactics); err != nil {
		return nil, err
	}

	if err := s.repo.Create(tactics); err != nil {
		return nil, err
	}
	detector.GetManager().Submit(detector.NewTacticsCreatedEvent(*tactics))
	return dto.ToTacticsResponse(tactics), nil
}

func (s *TacticsService) Get(tenantId string, tacticsId uint64) (*dto.TacticsResponse, error) {
	tactics, err := s.repo.Get(tenantId, tacticsId)
	if err != nil {
		return nil, err
	}
	return dto.ToTacticsResponse(tactics), nil
}

func (s *TacticsService) Update(tenantId string, tacticsId uint64, req *dto.TacticsUpdateRequest) (*dto.TacticsResponse, error) {
	tactics, err := s.repo.Get(tenantId, tacticsId)
	if err != nil {
		return nil, ErrTacticsNotFound
	}

	if req.Name != "" {
		tactics.Name = req.Name
	}
	if req.IntervalMs > 0 {
		tactics.IntervalMs = req.IntervalMs
	}
	if req.TimeoutMs > 0 {
		tactics.TimeoutMs = req.TimeoutMs
	}
	if req.UnstableMs > 0 {
		tactics.UnstableMs = req.UnstableMs
	}
	if req.Enabled != nil {
		tactics.Enabled = *req.Enabled
	}

	if err := validateTactics(tactics); err != nil {
		return nil, err
	}

	if err := s.repo.Update(tactics); err != nil {
		return nil, err
	}
	detector.GetManager().Submit(detector.NewTacticsUpdatedEvent(*tactics))
	return dto.ToTacticsResponse(tactics), nil
}

func validateTactics(t *model.Tactics) error {
	if t.Name == "" {
		return errors.New("策略名称不能为空")
	}
	if t.IntervalMs <= 0 {
		return errors.New("检测间隔必须大于 0")
	}
	if t.TimeoutMs <= 0 {
		return errors.New("超时时间必须大于 0")
	}
	if t.UnstableMs > t.TimeoutMs {
		return errors.New("不稳定阈值不能大于超时时间")
	}
	return nil
}

func (s *TacticsService) Delete(tenantId string, tacticsId uint64) error {
	count, err := s.ipRepo.CountByTactics(tenantId, tacticsId)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该策略组下存在绑定的 IP，无法删除")
	}

	rows, err := s.repo.Delete(tenantId, tacticsId)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrTacticsNotFound
	}
	detector.GetManager().Submit(detector.NewTacticsDeletedEvent(tacticsId))
	return nil
}

func (s *TacticsService) ListTenants() ([]string, error) {
	return s.repo.ListTenants()
}
