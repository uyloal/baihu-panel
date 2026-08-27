package services

import (
	"os"
	"path/filepath"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/services/deps"
	"github.com/uyloal/baihu-panel/internal/utils"
)

type InitService struct {
	settingsService *SettingsService
}

func NewInitService(settingsService *SettingsService) *InitService {
	return &InitService{
		settingsService: settingsService,
	}
}

// Initialize 执行系统初始化，返回 UserService
func (s *InitService) Initialize() *UserService {
	logger.Info("[Initialize] 开始初始化系统...")

	// 初始化默认设置
	if err := s.settingsService.InitSettings(); err != nil {
		logger.Warnf("[Initialize] 初始化设置失败: %v", err)
	}

	// 创建 UserService
	userService := NewUserService()

	// 创建管理员账号
	s.initializeAdmin(userService)

	// 初始化 Node.js & pnpm 脚本工作区
	s.initializeScriptsWorkspace()

	return userService
}

// initializeScriptsWorkspace 初始化 data 项目与内置 baihu 依赖，并确保 scripts 纯净脚本目录就绪
func (s *InitService) initializeScriptsWorkspace() {
	logger.Info("[Workspace] 正在检查与初始化 data Node 项目与 scripts 目录...")
	_ = os.MkdirAll(constant.ScriptsWorkDir, 0755)

	nodeManager := deps.NewNodeManager(constant.DataDir)
	if err := nodeManager.EnsurePackageJson(); err != nil {
		logger.Warnf("[Workspace] 初始化 data/package.json 失败: %v", err)
	} else {
		logger.Infof("[Workspace] 数据工作区已就绪: %s (脚本目录: %s)", filepath.Clean(constant.DataDir), filepath.Clean(constant.ScriptsWorkDir))
		_ = nodeManager.InstallBuiltinSdk()
	}
}

// initializeAdmin 创建管理员账号
func (s *InitService) initializeAdmin(userService *UserService) {
	existingUser := userService.GetUserByUsername("admin")
	if existingUser != nil {
		logger.Info("[Init] 管理员账号已存在，跳过创建")
		return
	}

	password := utils.RandomString(12)
	userService.CreateUser("admin", password, "admin@local", "admin")
	logger.Infof("--------------------------------------------------")
	logger.Infof("[Init] 管理员账号创建成功:")
	logger.Infof("[Init] 用户名: admin")
	logger.Infof("[Init] 密  码: %s", password)
	logger.Infof("[Init] 请妥善保管您的密码，并登录后及时修改。")
	logger.Infof("--------------------------------------------------")
}
