package builtininstall

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/uyloal/baihu-panel/cmd/clibase"
	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/services/deps"
	"github.com/uyloal/baihu-panel/internal/utils"
)

func printHelp() {
	clibase.PrintSubCommandUsage("白虎面板内建 SDK 依赖安装工具", "baihu builtininstall", "", nil)
}

// Run 执行内建包安装逻辑
func Run(args []string) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		printHelp()
		return
	}

	fs := flag.NewFlagSet("builtininstall", flag.ExitOnError)
	fs.Usage = printHelp

	if err := fs.Parse(args); err != nil {
		return
	}

	fmt.Println(">> [Builtin] 开始为 data 项目初始化与安装 baihu SDK...")

	dataDir := utils.ResolveAbsDataDir()
	scriptsDir := utils.ResolveAbsScriptsDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf(">> [Builtin] 创建 data 目录失败: %v\n", err)
		return
	}
	_ = os.MkdirAll(scriptsDir, 0755)

	nodeManager := deps.NewNodeManager(dataDir)
	if err := nodeManager.EnsurePackageJson(); err != nil {
		fmt.Printf(">> [Builtin] 初始化 package.json 失败: %v\n", err)
		return
	}

	// 查找 packages/baihu 路径
	baihuPkgPath := ""
	pwd, _ := os.Getwd()
	possiblePaths := []string{
		"/app/packages/baihu",
		"/www/packages/baihu",
		filepath.Join(constant.ResolveAppRootDir(), "packages", "baihu"),
		filepath.Join(pwd, "packages", "baihu"),
	}
	for _, p := range possiblePaths {
		if _, err := os.Stat(filepath.Join(p, "package.json")); err == nil {
			baihuPkgPath = p
			break
		}
	}

	var cmd *exec.Cmd
	if baihuPkgPath != "" {
		fmt.Printf(">> [Builtin] 正在从本地路径引入 baihu: %s\n", baihuPkgPath)
		cmd = exec.Command("pnpm", "add", baihuPkgPath)
	} else {
		fmt.Println(">> [Builtin] 正在执行 pnpm install...")
		cmd = exec.Command("pnpm", "install")
	}

	cmd.Dir = dataDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf(">> [Builtin] 安装内建 SDK 警告: %v\n输出: %s\n", err, string(out))
	} else {
		fmt.Printf(">> [Builtin] 内建 SDK 安装成功\n")
	}
}
