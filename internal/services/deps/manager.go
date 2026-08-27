package deps

import (
	"github.com/uyloal/baihu-panel/internal/models"
)

// Manager 依赖管理器接口
type Manager interface {
	Install(dep *models.Dependency) error
	Uninstall(dep *models.Dependency) error
	GetInstalledPackages() ([]models.Dependency, error)
	GetInstallCommand(dep *models.Dependency) (string, error)
	GetBatchInstallCommand(deps []models.Dependency) (string, error)
	GetReinstallAllCommand(deps []models.Dependency) (string, error)
	GetVerifyCommand() (string, error)
}

// GetManager 获取全局 Node.js / pnpm 依赖管理器
func GetManager() Manager {
	return DefaultNodeManager
}
