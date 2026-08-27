package deps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/utils"
)

type NodeManager struct {
	WorkDir string
}

var DefaultNodeManager = NewNodeManager("")

func NewNodeManager(workDir string) *NodeManager {
	if workDir == "" {
		workDir = constant.DataDir
	}
	return &NodeManager{
		WorkDir: workDir,
	}
}

func (m *NodeManager) getDataDir() string {
	if m.WorkDir != "" {
		return m.WorkDir
	}
	return utils.ResolveAbsDataDir()
}

// EnsurePackageJson 确保 data 根目录存在且包含初始 package.json
func (m *NodeManager) EnsurePackageJson() error {
	dir := m.getDataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	pkgPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		initialContent := `{
  "name": "baihu-data",
  "version": "1.0.0",
  "type": "module",
  "private": true,
  "description": "Node.js environment for Baihu Panel"
}
`
		if err := os.WriteFile(pkgPath, []byte(initialContent), 0644); err != nil {
			return err
		}
	}

	tsconfigContent := `{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "Node",
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "skipLibCheck": true
  }
}
`
	tsconfigPath := filepath.Join(dir, "tsconfig.json")
	if _, err := os.Stat(tsconfigPath); os.IsNotExist(err) {
		_ = os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)
	}

	scriptsDir := utils.ResolveAbsScriptsDir()
	if scriptsDir != "" && scriptsDir != dir {
		_ = os.MkdirAll(scriptsDir, 0755)
	}

	return nil
}

// InstallBuiltinSdk 使用 pnpm add 添加本地内置 baihu SDK 到 data 工作区
func (m *NodeManager) InstallBuiltinSdk() error {
	if err := m.EnsurePackageJson(); err != nil {
		return err
	}

	dir := m.getDataDir()
	baihuPkgPath := ""
	possiblePaths := []string{
		"/app/packages/baihu",
		"/www/packages/baihu",
		filepath.Join(constant.ResolveAppRootDir(), "packages", "baihu"),
	}
	for _, p := range possiblePaths {
		if _, err := os.Stat(filepath.Join(p, "package.json")); err == nil {
			baihuPkgPath = p
			break
		}
	}

	if baihuPkgPath == "" {
		return nil
	}

	cmd := exec.Command("pnpm", "add", baihuPkgPath)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warnf("[Workspace] 安装内建 SDK 警告: %v, 输出: %s", err, string(out))
		return err
	}
	logger.Infof("[Workspace] 内建 SDK 已通过 pnpm add 成功添加至 data 项目")
	return nil
}

func (m *NodeManager) runCmd(cmdArgs []string) ([]byte, error) {
	if err := m.EnsurePackageJson(); err != nil {
		return nil, err
	}

	dir := m.getDataDir()
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	return cmd.CombinedOutput()
}

func (m *NodeManager) Install(dep *models.Dependency) error {
	var packageSpec string
	if dep.Version != "" {
		packageSpec = dep.Name + "@" + dep.Version
	} else {
		packageSpec = dep.Name
	}

	args := []string{"pnpm", "add", packageSpec}
	logger.Infof("[pnpm] 正在 data 目录下安装 Node 依赖: %s", packageSpec)

	output, err := m.runCmd(args)
	dep.Log = models.BigText(output)

	if err != nil {
		logger.Errorf("[pnpm] 安装失败: %v, 输出:\n%s", err, string(output))
		return errors.New("安装失败: " + string(output))
	}
	logger.Infof("[pnpm] 安装成功: %s", packageSpec)
	return nil
}

func (m *NodeManager) Uninstall(dep *models.Dependency) error {
	args := []string{"pnpm", "remove", dep.Name}
	logger.Infof("[pnpm] 正在 data 目录下卸载 Node 依赖: %s", dep.Name)

	output, err := m.runCmd(args)
	if err != nil {
		logger.Errorf("[pnpm] 卸载失败: %v, 输出:\n%s", err, string(output))
		return errors.New("卸载失败: " + string(output))
	}
	logger.Infof("[pnpm] 卸载成功: %s", dep.Name)
	return nil
}

func (m *NodeManager) GetInstalledPackages() ([]models.Dependency, error) {
	pkgPath := filepath.Join(m.getDataDir(), "package.json")
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return []models.Dependency{}, nil
	}

	var pkg PackageJson
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, err
	}

	var packages []models.Dependency
	for name, ver := range pkg.Dependencies {
		cleanVer := strings.TrimLeft(ver, "^~>=<* ")
		packages = append(packages, models.Dependency{
			Name:     name,
			Version:  cleanVer,
			Language: "node",
			Remark:   "dependencies",
		})
	}
	for name, ver := range pkg.DevDependencies {
		cleanVer := strings.TrimLeft(ver, "^~>=<* ")
		packages = append(packages, models.Dependency{
			Name:     name,
			Version:  cleanVer,
			Language: "node",
			Remark:   "devDependencies",
		})
	}

	return packages, nil
}

func (m *NodeManager) GetInstallCommand(dep *models.Dependency) (string, error) {
	var packageSpec string
	if dep.Version != "" {
		packageSpec = dep.Name + "@" + dep.Version
	} else {
		packageSpec = dep.Name
	}

	dir := utils.ResolveAbsDataDir()
	return fmt.Sprintf("cd %q && pnpm add %s", dir, packageSpec), nil
}

func (m *NodeManager) GetBatchInstallCommand(deps []models.Dependency) (string, error) {
	if len(deps) == 0 {
		return "echo \"没有需要安装的依赖\"", nil
	}

	var packageSpecs []string
	for _, dep := range deps {
		if dep.Version != "" {
			packageSpecs = append(packageSpecs, dep.Name+"@"+dep.Version)
		} else {
			packageSpecs = append(packageSpecs, dep.Name)
		}
	}

	dir := utils.ResolveAbsDataDir()
	return fmt.Sprintf("cd %q && pnpm add %s", dir, strings.Join(packageSpecs, " ")), nil
}

func (m *NodeManager) GetReinstallAllCommand(deps []models.Dependency) (string, error) {
	dir := utils.ResolveAbsDataDir()
	return fmt.Sprintf("cd %q && pnpm install", dir), nil
}

func (m *NodeManager) GetVerifyCommand() (string, error) {
	return "node -v && pnpm -v", nil
}
