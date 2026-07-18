package repository

import (
	"sync"

	"bs-net-monitor/internal/model"
)

// TacticsRepository 负责策略组数据的读写操作。
type TacticsRepository struct{}

var (
	tacticsRepositoryInstance *TacticsRepository
	tacticsRepositoryOnce     sync.Once
)

// GetTacticsRepository 返回策略组仓库的单例。
func GetTacticsRepository() *TacticsRepository {
	tacticsRepositoryOnce.Do(func() {
		tacticsRepositoryInstance = &TacticsRepository{}
	})
	return tacticsRepositoryInstance
}

func (r *TacticsRepository) List(tenantId string) ([]model.Tactics, error) {
	var list []model.Tactics
	err := db.Where("tenant_id = ?", tenantId).Find(&list).Error
	return list, err
}
func (r *TacticsRepository) Get(tenantId string, tacticsId uint64) (*model.Tactics, error) {
	var tactics model.Tactics
	err := db.Where("tactics_id = ? AND tenant_id = ?", tacticsId, tenantId).First(&tactics).Error
	if err != nil {
		return nil, err
	}
	return &tactics, nil
}

func (r *TacticsRepository) Create(tactics *model.Tactics) error {
	return db.Create(tactics).Error
}

func (r *TacticsRepository) Update(tactics *model.Tactics) error {
	return db.Save(tactics).Error
}

func (r *TacticsRepository) Delete(tenantId string, tacticsId uint64) (int64, error) {
	result := db.Where("tactics_id = ? AND tenant_id = ?", tacticsId, tenantId).Delete(&model.Tactics{})
	return result.RowsAffected, result.Error
}

func (r *TacticsRepository) ListAllEnabled() ([]model.Tactics, error) {
	var list []model.Tactics
	err := db.Where("enabled = ?", true).Find(&list).Error
	return list, err
}

func (r *TacticsRepository) ListTenants() ([]string, error) {
	var tenants []string
	err := db.Model(&model.Tactics{}).Distinct("tenant_id").Pluck("tenant_id", &tenants).Error
	return tenants, err
}
