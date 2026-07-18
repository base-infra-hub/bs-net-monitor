package service

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/xuri/excelize/v2"

	"bs-net-monitor/internal/detector"
	"bs-net-monitor/internal/dto"
	"bs-net-monitor/internal/model"
	"bs-net-monitor/internal/repository"
)

var (
	ErrIPNotFound      = errors.New("IP 不存在")
	ErrTacticsNotFound = errors.New("策略组不存在")
)

// IPService 处理 IP 相关的业务逻辑。
type IPService struct {
	repo        *repository.IPRepository
	tacticsRepo *repository.TacticsRepository
}

var (
	ipServiceInstance *IPService
	ipServiceOnce     sync.Once
)

// GetIPService 返回 IP 服务的单例。
func GetIPService() *IPService {
	ipServiceOnce.Do(func() {
		ipServiceInstance = &IPService{
			repo:        repository.GetIPRepository(),
			tacticsRepo: repository.GetTacticsRepository(),
		}
	})
	return ipServiceInstance
}

func (s *IPService) tacticsNameMap(tenantId string) (map[uint64]string, error) {
	tacticsList, err := s.tacticsRepo.List(tenantId)
	if err != nil {
		return nil, err
	}
	m := make(map[uint64]string, len(tacticsList))
	for i := range tacticsList {
		m[tacticsList[i].TacticsId] = tacticsList[i].Name
	}
	return m, nil
}

func (s *IPService) List(tenantId string, query dto.IPListQuery) (*dto.PageRes[dto.IPResponse], error) {
	total, ips, err := s.repo.List(tenantId, query.TacticsId, query.Enabled, query.Current, query.Size)
	if err != nil {
		return nil, err
	}

	tacticsMap, err := s.tacticsNameMap(tenantId)
	if err != nil {
		return nil, err
	}

	pages := int(total) / query.Size
	if int(total)%query.Size > 0 {
		pages++
	}

	return &dto.PageRes[dto.IPResponse]{
		Total:   total,
		Records: dto.ToIPResponseList(ips, tacticsMap),
		Current: query.Current,
		Size:    query.Size,
		Pages:   pages,
	}, nil
}

func (s *IPService) Create(tenantId string, req *dto.IPCreateRequest) (*dto.IPResponse, error) {
	ip := &model.IP{
		TenantId:  tenantId,
		Name:      req.Name,
		Ip:        req.Ip,
		Position:  req.Position,
		Remark:    req.Remark,
		TacticsId: req.TacticsId,
		Enabled:   req.Enabled,
	}

	if err := s.repo.Create(ip); err != nil {
		return nil, err
	}
	detector.GetManager().Submit(detector.NewIPBatchChangedEvent([]model.IP{*ip}))

	tacticsMap, _ := s.tacticsNameMap(tenantId)
	return dto.ToIPResponse(ip, tacticsMap[ip.TacticsId]), nil
}

func (s *IPService) Get(tenantId string, ipId uint64) (*dto.IPResponse, error) {
	ip, err := s.repo.Get(tenantId, ipId)
	if err != nil {
		return nil, err
	}
	tacticsMap, _ := s.tacticsNameMap(tenantId)
	return dto.ToIPResponse(ip, tacticsMap[ip.TacticsId]), nil
}

func (s *IPService) Update(tenantId string, ipId uint64, req *dto.IPUpdateRequest) (*dto.IPResponse, error) {
	ip, err := s.repo.Get(tenantId, ipId)
	if err != nil {
		return nil, ErrIPNotFound
	}

	if req.Name != "" {
		ip.Name = req.Name
	}
	if req.Ip != "" {
		ip.Ip = req.Ip
	}
	ip.Position = req.Position
	ip.Remark = req.Remark
	if req.TacticsId != 0 {
		ip.TacticsId = req.TacticsId
	}
	if req.Enabled != nil {
		ip.Enabled = *req.Enabled
	}

	if err := s.repo.Update(ip); err != nil {
		return nil, err
	}
	detector.GetManager().Submit(detector.NewIPBatchChangedEvent([]model.IP{*ip}))

	tacticsMap, _ := s.tacticsNameMap(tenantId)
	return dto.ToIPResponse(ip, tacticsMap[ip.TacticsId]), nil
}

func (s *IPService) Delete(tenantId string, ipId uint64) error {
	rows, err := s.repo.Delete(tenantId, ipId)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrIPNotFound
	}
	detector.GetManager().Submit(detector.NewIPBatchDeletedEvent([]uint64{ipId}))
	return nil
}

func (s *IPService) ListTenants() ([]string, error) {
	return s.repo.ListTenants()
}

func (s *IPService) BatchUpdateEnabled(tenantId string, req *dto.IPBatchUpdateRequest) error {
	if len(req.IpIds) == 0 {
		return errors.New("ipIds 为空")
	}
	_, err := s.repo.BatchUpdateEnabled(tenantId, req.IpIds, req.Enabled)
	if err != nil {
		return err
	}
	ips, err := s.repo.ListByIDs(tenantId, req.IpIds)
	if err != nil {
		return err
	}
	detector.GetManager().Submit(detector.NewIPBatchChangedEvent(ips))
	return nil
}

func (s *IPService) BatchDelete(tenantId string, req *dto.IPBatchDeleteRequest) error {
	if len(req.IpIds) == 0 {
		return errors.New("ipIds 为空")
	}
	_, err := s.repo.BatchDelete(tenantId, req.IpIds)
	if err != nil {
		return err
	}
	detector.GetManager().Submit(detector.NewIPBatchDeletedEvent(req.IpIds))
	return nil
}

func (s *IPService) ImportIPs(tenantId string, reader io.Reader, tacticsId uint64) (int, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, err
	}

	var ips []model.IP
	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}
		if len(row) < 2 {
			continue
		}
		name := row[0]
		ip := row[1]
		if name == "" || ip == "" {
			continue
		}

		var position, remark string
		if len(row) > 2 {
			position = row[2]
		}
		if len(row) > 3 {
			remark = row[3]
		}

		ips = append(ips, model.IP{
			TenantId:  tenantId,
			Name:      name,
			Ip:        ip,
			Position:  position,
			Remark:    remark,
			TacticsId: tacticsId,
			Enabled:   true,
		})
	}

	if len(ips) == 0 {
		return 0, errors.New("未找到有效的 IP 行")
	}

	if err := s.repo.BatchCreate(ips); err != nil {
		return 0, err
	}
	detector.GetManager().Submit(detector.NewIPBatchChangedEvent(ips))
	return len(ips), nil
}

func (s *IPService) ExportIPs(tenantId string, tacticsId uint64, w io.Writer) error {
	ips, err := s.repo.ListAll(tenantId, tacticsId)
	if err != nil {
		return err
	}

	tacticsMap, err := s.tacticsNameMap(tenantId)
	if err != nil {
		tacticsMap = make(map[uint64]string)
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "IP列表"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	// 写入表头 (除了 ID，其余字段都导出)
	headers := []string{"设备名称", "IP地址", "物理位置", "备注信息", "关联策略组", "启用状态", "录入时间", "更新时间"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// 写入数据
	for i, ip := range ips {
		row := i + 2

		// 关联策略组名称
		tacticsName := tacticsMap[ip.TacticsId]
		if tacticsName == "" {
			tacticsName = "未关联"
		}

		// 启用状态
		enabledText := "已停用"
		if ip.Enabled {
			enabledText = "已启用"
		}

		// 格式化时间
		createdAtText := ip.CreatedAt.Local().Format("2006-01-02 15:04:05")
		updatedAtText := ip.UpdatedAt.Local().Format("2006-01-02 15:04:05")

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), ip.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), ip.Ip)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), ip.Position)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), ip.Remark)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), tacticsName)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), enabledText)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), createdAtText)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), updatedAtText)
	}

	return f.Write(w)
}

func (s *IPService) GetImportTemplate(w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "IP导入模板"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	// 写入表头
	headers := []string{"设备名称", "IP地址", "物理位置", "备注信息"}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	return f.Write(w)
}
