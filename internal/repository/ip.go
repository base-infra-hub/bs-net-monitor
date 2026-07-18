package repository

import (
	"sync"

	"bs-net-monitor/internal/model"

	"gorm.io/gorm"
)

var db *gorm.DB

func Init(gormDB *gorm.DB) {
	db = gormDB
}

// IPRepository 负责 IP 数据的读写操作。
type IPRepository struct{}

var (
	ipRepositoryInstance *IPRepository
	ipRepositoryOnce     sync.Once
)

// GetIPRepository 返回 IP 仓库的单例。
func GetIPRepository() *IPRepository {
	ipRepositoryOnce.Do(func() {
		ipRepositoryInstance = &IPRepository{}
	})
	return ipRepositoryInstance
}

func (r *IPRepository) List(tenantId string, tacticsId uint64, enabled *bool, current, size int) (int64, []model.IP, error) {
	query := db.Model(&model.IP{}).Where("tenant_id = ?", tenantId)
	if tacticsId > 0 {
		query = query.Where("tactics_id = ?", tacticsId)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var ips []model.IP
	offset := (current - 1) * size
	err := query.Order("ip_id DESC").Limit(size).Offset(offset).Find(&ips).Error
	return total, ips, err
}
func (r *IPRepository) All() ([]model.IP, error) {
	var ips []model.IP
	err := db.Find(&ips).Error
	return ips, err
}
func (r *IPRepository) ListAll(tenantId string, tacticsId uint64) ([]model.IP, error) {
	var ips []model.IP
	query := db.Where("tenant_id = ?", tenantId)
	if tacticsId > 0 {
		query = query.Where("tactics_id = ?", tacticsId)
	}
	err := query.Order("ip_id DESC").Find(&ips).Error
	return ips, err
}
func (r *IPRepository) Get(tenantId string, ipId uint64) (*model.IP, error) {
	var ip model.IP
	err := db.Where("ip_id = ? AND tenant_id = ?", ipId, tenantId).First(&ip).Error
	if err != nil {
		return nil, err
	}
	return &ip, nil
}

func (r *IPRepository) Create(ip *model.IP) error {
	return db.Create(ip).Error
}

func (r *IPRepository) BatchCreate(ips []model.IP) error {
	return db.CreateInBatches(ips, 100).Error
}

func (r *IPRepository) Update(ip *model.IP) error {
	return db.Save(ip).Error
}

func (r *IPRepository) Delete(tenantId string, ipId uint64) (int64, error) {
	result := db.Where("ip_id = ? AND tenant_id = ?", ipId, tenantId).Delete(&model.IP{})
	return result.RowsAffected, result.Error
}

func (r *IPRepository) CountByTactics(tenantId string, tacticsId uint64) (int64, error) {
	var count int64
	err := db.Model(&model.IP{}).Where("tenant_id = ? AND tactics_id = ?", tenantId, tacticsId).Count(&count).Error
	return count, err
}

func (r *IPRepository) ListByTactics(tenantId string, tacticsId uint64) ([]model.IP, error) {
	var ips []model.IP
	err := db.Where("tenant_id = ? AND tactics_id = ?", tenantId, tacticsId).Find(&ips).Error
	return ips, err
}

func (r *IPRepository) ListAllEnabled() ([]model.IP, error) {
	var ips []model.IP
	err := db.Where("enabled = ?", true).Find(&ips).Error
	return ips, err
}

func (r *IPRepository) ListByIDs(tenantId string, ipIds []uint64) ([]model.IP, error) {
	var ips []model.IP
	err := db.Where("tenant_id = ? AND ip_id IN ?", tenantId, ipIds).Find(&ips).Error
	return ips, err
}

func (r *IPRepository) ListTenants() ([]string, error) {
	var tenants []string
	err := db.Model(&model.IP{}).Distinct("tenant_id").Pluck("tenant_id", &tenants).Error
	return tenants, err
}

func (r *IPRepository) BatchUpdateEnabled(tenantId string, ipIds []uint64, enabled bool) (int64, error) {
	result := db.Model(&model.IP{}).Where("tenant_id = ? AND ip_id IN ?", tenantId, ipIds).Update("enabled", enabled)
	return result.RowsAffected, result.Error
}

func (r *IPRepository) BatchDelete(tenantId string, ipIds []uint64) (int64, error) {
	result := db.Where("tenant_id = ? AND ip_id IN ?", tenantId, ipIds).Delete(&model.IP{})
	return result.RowsAffected, result.Error
}
