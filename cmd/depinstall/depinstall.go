package depinstall

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/uyloal/baihu-panel/cmd/clibase"
	"github.com/uyloal/baihu-panel/internal/database"
	"github.com/uyloal/baihu-panel/internal/models"
	"github.com/uyloal/baihu-panel/internal/services"
	"github.com/uyloal/baihu-panel/internal/services/deps"
	"github.com/uyloal/baihu-panel/internal/utils"
)

// Run 依赖自动补全命令入口 (Node.js & pnpm)
func Run(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: baihu depinstall <log_id>")
		return
	}

	logID := args[0]
	fmt.Println(">> 提示: 白虎面板 Node.js 依赖自动补全工具")

	// 初始化基础环境和数据库连接
	if err := clibase.InitContext(true); err != nil {
		fmt.Printf(">> 初始化环境失败: %v\n", err)
		return
	}

	var log models.TaskLog
	if err := database.DB.Where("id = ?", logID).First(&log).Error; err != nil {
		fmt.Printf(">> 未找到指定的任务日志 (ID: %s): %v\n", logID, err)
		return
	}

	logOutput, err := utils.DecompressFromBase64(string(log.Output))
	if err != nil {
		fmt.Printf(">> 解压日志失败: %v\n", err)
		return
	}

	detected, found := deps.DetectMissingDependencies(logOutput)
	reader := bufio.NewReader(os.Stdin)

	var pkgsToInstall []string
	if !found || len(detected) == 0 {
		fmt.Println(">> 分析完毕: 未从最近一次的任务运行日志中检测到缺失依赖模式。")
		fmt.Println(">> 您可以手动输入想要安装的 npm 依赖包名称（多个包用空格分隔，若不安装请直接回车退出）:")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println(">> 已退出依赖补全。")
			return
		}
		pkgsToInstall = strings.Fields(input)
	} else {
		fmt.Println(">> 分析结果: 从运行日志中检测到以下缺失 Node 依赖包：")
		fmt.Printf("   %s\n", strings.Join(detected, ", "))
		fmt.Println(">> 是否确认自动安装上述依赖包？(y/N):")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println(">> 用户已取消安装操作。")
			return
		}
		pkgsToInstall = detected
	}

	fmt.Println("==================================================================")
	fmt.Println(">> 开始执行依赖安装，请稍候...")
	fmt.Println("==================================================================")

	var failedPkgs []string
	depService := services.NewDependencyService()
	nodeManager := deps.GetManager()

	for _, pkg := range pkgsToInstall {
		dep := &models.Dependency{
			Name:     pkg,
			Language: "node",
		}

		cmdStr, err := nodeManager.GetInstallCommand(dep)
		if err != nil {
			fmt.Printf(">> 无法生成依赖包 [%s] 的安装命令: %v\n", pkg, err)
			failedPkgs = append(failedPkgs, pkg)
			continue
		}

		fmt.Printf(">> 正在安装 [%s] -> 执行指令: %s\n", pkg, cmdStr)

		execCmd := utils.NewShellCommandCmd(cmdStr)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		runErr := execCmd.Run()
		if runErr != nil {
			fmt.Printf(">> 【失败】依赖包 [%s] 安装出错。\n\n", pkg)
			failedPkgs = append(failedPkgs, pkg)
		} else {
			fmt.Printf(">> 【成功】依赖包 [%s] 安装成功！\n\n", pkg)
			_ = depService.Create(dep)
		}
	}

	fmt.Println("==================================================================")
	if len(failedPkgs) > 0 {
		fmt.Printf(">> 依赖补全已结束。其中以下依赖包安装失败，请用户自行判断/手动处理：\n")
		for _, fp := range failedPkgs {
			fmt.Printf("   - %s\n", fp)
		}
	} else {
		fmt.Println(">> 恭喜！所有依赖包安装成功！")
	}
	fmt.Println("==================================================================")
}
