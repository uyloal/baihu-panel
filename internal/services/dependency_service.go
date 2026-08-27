package services

import (
	"errors"

	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services/deps"
	"github.com/uyloal/baihu-panel/internal/utils"
)

type DependencyService struct {
	manager deps.Manager
}

func NewDependencyService() *DependencyService {
	return &DependencyService{
		manager: deps.GetManager(),
	}
}

// List 获取依赖列表
func (s *DependencyService) List() ([]models.Dependency, error) {
	var results []models.Dependency
	err := database.DB.Order("id desc").Find(&results).Error
	return results, err
}

// Create 创建依赖记录
func (s *DependencyService) Create(dep *models.Dependency) error {
	var existing models.Dependency
	res := database.DB.Where("name = ?", dep.Name).Limit(1).Find(&existing)
	if res.Error == nil && res.RowsAffected > 0 {
		dep.ID = existing.ID
		return database.DB.Model(&existing).Updates(dep).Error
	}

	if dep.ID == "" {
		dep.ID = utils.GenerateID()
	}
	if dep.Language == "" {
		dep.Language = "node"
	}
	return database.DB.Create(dep).Error
}

// Delete 删除依赖记录
func (s *DependencyService) Delete(id string) error {
	return database.DB.Where("id = ?", id).Delete(&models.Dependency{}).Error
}

// Install 安装依赖
func (s *DependencyService) Install(dep *models.Dependency) error {
	if s.manager == nil {
		return errors.New("依赖管理器未就绪")
	}
	return s.manager.Install(dep)
}

// Uninstall 卸载依赖
func (s *DependencyService) Uninstall(dep *models.Dependency) error {
	if s.manager == nil {
		return errors.New("依赖管理器未就绪")
	}
	return s.manager.Uninstall(dep)
}

// GetInstalledPackages 获取 scripts 工程已安装的包列表 (读取 package.json)
func (s *DependencyService) GetInstalledPackages() ([]models.Dependency, error) {
	if s.manager == nil {
		return nil, errors.New("依赖管理器未就绪")
	}
	return s.manager.GetInstalledPackages()
}

// GetInstallCommand 获取安装命令
func (s *DependencyService) GetInstallCommand(dep *models.Dependency) (string, error) {
	if s.manager == nil {
		return "", errors.New("依赖管理器未就绪")
	}
	return s.manager.GetInstallCommand(dep)
}

// GetReinstallAllCommand 获取全部重装命令
func (s *DependencyService) GetReinstallAllCommand() (string, error) {
	if s.manager == nil {
		return "", errors.New("依赖管理器未就绪")
	}
	depsList, err := s.List()
	if err != nil {
		return "", err
	}
	return s.manager.GetReinstallAllCommand(depsList)
}

// GetVerifyCommand 获取环境验证命令
func (s *DependencyService) GetVerifyCommand() (string, error) {
	if s.manager == nil {
		return "", errors.New("依赖管理器未就绪")
	}
	return s.manager.GetVerifyCommand()
}

// GetBatchInstallCommand 获取批量安装命令
func (s *DependencyService) GetBatchInstallCommand(depsList []models.Dependency) (string, error) {
	if len(depsList) == 0 {
		return "", errors.New("依赖包列表不能为空")
	}
	if s.manager == nil {
		return "", errors.New("依赖管理器未就绪")
	}
	return s.manager.GetBatchInstallCommand(depsList)
}

// ImportDependencies 批量导入依赖并自动入库去重
func (s *DependencyService) ImportDependencies(depsList []models.Dependency) ([]models.Dependency, error) {
	var imported []models.Dependency
	for i := range depsList {
		dep := &depsList[i]
		if dep.Language == "" {
			dep.Language = "node"
		}
		if err := s.Create(dep); err == nil {
			imported = append(imported, *dep)
		}
	}
	return imported, nil
}
