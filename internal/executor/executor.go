package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/logger"
	"github.com/uyloal/baihu-panel/internal/utils"
	"github.com/uyloal/baihu-panel/internal/windows"
)

// Task 任务基础接口
type Task interface {
	GetID() string
	GetName() string
	GetCommand() string
	GetPreCommand() string
	GetPostCommand() string
	GetTimeout() int
	GetWorkDir() string
	GetEnvs() string
	GetEnvVars() []string
}

// CronTask 计划任务接口
type CronTask interface {
	Task
	GetSchedule() string
	GetSecrets() []string
	GetRandomRange() int
}

// Request 任务执行请求
type Request struct {
	Command     string
	PreCommand  string
	PostCommand string
	WorkDir     string
	Envs        []string
	Timeout     int // 任务超时时间（分钟）
}

// Result 任务执行结果
type Result struct {
	Output    string
	Error     string
	Status    string // 状态: success, failed
	Duration  int64  // 毫秒
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
}

// Hooks 执行钩子接口
type Hooks interface {
	// PreExecute 执行前钩子，返回日志ID和错误
	PreExecute(ctx context.Context, req Request) (logID string, err error)

	// PostExecute 执行后钩子，处理日志压缩和记录更新
	PostExecute(ctx context.Context, logID string, result *Result) error

	// OnHeartbeat 执行中心跳钩子，用于更新实时状态
	OnHeartbeat(ctx context.Context, logID string, duration int64) error
}

// Execute 执行命令（基础版本，不带钩子）
func Execute(ctx context.Context, req Request, stdout, stderr io.Writer) (*Result, error) {
	return ExecuteWithHooks(ctx, req, stdout, stderr, nil)
}

// ExecuteWithHooks 执行命令（带钩子支持）
func ExecuteWithHooks(ctx context.Context, req Request, stdout, stderr io.Writer, hooks Hooks) (res *Result, err error) {
	start := time.Now()

	// 捕获任务执行过程中的 Panic 异常并输出 Go 层面的堆栈信息 (Stack Trace)
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			logger.Errorf("[Executor] 任务执行发生 Panic 异常: %v\n%s", r, stackTrace)
			err = fmt.Errorf("系统 Runtime Panic: %v", r)
			res = &Result{
				Status:    constant.TaskStatusFailed,
				Error:     err.Error(),
				Duration:  time.Since(start).Milliseconds(),
				ExitCode:  1,
				StartTime: start,
				EndTime:   time.Now(),
			}
			writeDiagnosticError(stdout, start, req.WorkDir, req.Command, false, err.Error(), 1, stackTrace)
		}
	}()

	// 演示模式拦截
	if constant.DemoMode {
		logger.Warnf("[Executor] 演示模式下已拦截命令执行: %s", req.Command)
		if stdout != nil {
			stdout.Write([]byte("\r\n\033[1;33m[演示模式] 命令执行已跳过\033[0m\r\n"))
		}

		// 仍然触发 PreExecute 以便流程完整
		var logID string
		if hooks != nil {
			logID, _ = hooks.PreExecute(ctx, req)
		}

		result := &Result{
			Status:    constant.TaskStatusFailed,
			Output:    "[演示模式] 该任务在演示模式下被禁用执行",
			StartTime: start,
			EndTime:   time.Now(),
		}

		if hooks != nil {
			hooks.PostExecute(ctx, logID, result)
		}
		return result, nil
	}

	// 2. 执行命令
	timeout := req.Timeout
	var execCtx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// 组合指令（如果存在前置或后置指令）
	if req.PreCommand != "" || req.PostCommand != "" {
		finalCmd := ""
		if req.PreCommand != "" {
			finalCmd += req.PreCommand + "\n"
		}
		finalCmd += req.Command
		if req.PostCommand != "" {
			finalCmd += "\n" + req.PostCommand
		}
		req.Command = finalCmd
	}

	// 1. 执行前钩子
	var logID string
	if hooks != nil {
		id, hookErr := hooks.PreExecute(ctx, req)
		if hookErr != nil {
			return &Result{
				Status:    constant.TaskStatusFailed,
				Duration:  0,
				ExitCode:  1,
				StartTime: start,
				EndTime:   time.Now(),
			}, hookErr
		}
		logID = id
	}

	shell, args := utils.GetShellCommand(req.Command)
	cmd := exec.CommandContext(execCtx, shell, args...)

	usePty := !windows.IsWindows() && stdout != nil && (stdout == stderr || stdout == io.Discard)
	SetProcessGroupAndCancel(cmd, usePty)

	if !usePty {
		cmd.Stdin = strings.NewReader("")
	}

	// 设置工作目录（默认使用 scripts 目录）
	workDir := strings.TrimSpace(req.WorkDir)
	if workDir != "" {
		cmd.Dir = workDir
	} else {
		cmd.Dir = constant.ScriptsWorkDir
		workDir = constant.ScriptsWorkDir
	}

	// 设置环境变量（始终继承系统环境变量）
	cmd.Env = os.Environ()
	if len(req.Envs) > 0 {
		cmd.Env = append(cmd.Env, req.Envs...)
	}
	// 针对 Windows 平台修复 PATH 优先级
	cmd.Env = windows.FixPathEnv(cmd.Env)
	// 注入 Node 与终端环境优化参数
	cmd.Env = append(cmd.Env,
		"TERM=xterm",
		"NODE_NO_WARNINGS=1",
	)

	hasNodeOptions := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "NODE_OPTIONS=") {
			hasNodeOptions = true
			break
		}
	}
	if !hasNodeOptions {
		cmd.Env = append(cmd.Env, "NODE_OPTIONS=--experimental-strip-types --max-old-space-size=512")
	}

	var pipeReader *os.File
	var pipeWriter *os.File
	var ptyFile *os.File
	var copyDone chan struct{}

	var started bool
	// 尝试开启 PTY 模式（Unix/macOS 且输出合并时）
	if !windows.IsWindows() && stdout != nil && (stdout == stderr || stdout == io.Discard) {
		f, ptyErr := pty.Start(cmd)
		if ptyErr == nil {
			logger.Infof("[Executor] #%s 启动于 PTY 模式", logID)
			ptyFile = f
			started = true
			copyDone = make(chan struct{})
			go func() {
				defer close(copyDone)
				io.Copy(stdout, f)
				f.Close()
			}()
		} else {
			logger.Warnf("[Executor] 任务 #%s PTY 启动失败，正在回退至管道(Pipe)模式: %v", logID, ptyErr)
			newCmd := exec.CommandContext(execCtx, shell, args...)
			newCmd.Stdin = strings.NewReader("")
			if workDir != "" {
				newCmd.Dir = workDir
			}
			newCmd.Env = cmd.Env
			SetProcessGroupAndCancel(newCmd, false)
			cmd = newCmd
		}
	}

	if !started {
		if stdout != stderr && stdout != io.Discard {
			logger.Debugf("[Executor] 任务 #%s stdout 和 stderr 不同，回退到 Pipe 模式。", logID)
		}
		logger.Infof("[Executor] #%s 启动于 Pipe 模式", logID)
		if stdout != nil && stdout == stderr {
			pr, pw, pipeErr := os.Pipe()
			if pipeErr == nil {
				cmd.Stdout = pw
				cmd.Stderr = pw
				pipeReader = pr
				pipeWriter = pw
				copyDone = make(chan struct{})
				go func() {
					io.Copy(stdout, pr)
					pr.Close()
					close(copyDone)
				}()
			} else {
				cmd.Stdout = stdout
				cmd.Stderr = stderr
			}
		} else {
			cmd.Stdout = stdout
			cmd.Stderr = stderr
		}

		err = cmd.Start()
		if err != nil {
			if pipeWriter != nil {
				pipeWriter.Close()
			}
			if pipeReader != nil {
				pipeReader.Close()
			}
			writeDiagnosticError(stdout, start, workDir, req.Command, usePty, fmt.Sprintf("进程 fork/exec 启动失败: %v", err), 1, "")

			end := time.Now()
			result := &Result{
				Status:    constant.TaskStatusFailed,
				Duration:  end.Sub(start).Milliseconds(),
				ExitCode:  1,
				StartTime: start,
				EndTime:   end,
			}
			if hooks != nil {
				result.Output += "\n[系统错误] " + err.Error()
				hooks.PostExecute(ctx, logID, result)
			}
			return result, err
		}

		if pipeWriter != nil {
			pipeWriter.Close()
		}
	}

	// 启动心跳协程
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if hooks != nil {
					hooks.OnHeartbeat(ctx, logID, time.Since(start).Milliseconds())
				}
			}
		}
	}()

	// 等待命令完成
	err = cmd.Wait()
	close(done) // 停止心跳

	// PTY 模式下显式关闭
	if ptyFile != nil {
		ptyFile.Close()
	}

	// 强制关闭管道读端
	if pipeReader != nil {
		pipeReader.Close()
	}

	// 等待日志复制完成
	if copyDone != nil {
		<-copyDone
	}

	end := time.Now()

	result := &Result{
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start).Milliseconds(),
	}

	if err != nil {
		result.Status = constant.TaskStatusFailed
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
	} else {
		result.Status = constant.TaskStatusSuccess
		result.ExitCode = 0
	}

	// 仅在任务执行失败/异常退出时向日志流追加写入诊断尾部
	if result.Status != constant.TaskStatusSuccess {
		var stack string
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			stack = string(exitErr.Stderr)
		}
		writeDiagnosticError(stdout, start, workDir, req.Command, usePty, result.Error, result.ExitCode, stack)
	}

	// 3. 执行后钩子
	if hooks != nil {
		if hookErr := hooks.PostExecute(ctx, logID, result); hookErr != nil {
			result.Output += "\n[钩子错误] " + hookErr.Error()
		}
	}

	return result, err
}

// writeDiagnosticError 统一向 stdout 追加格式化的诊断失败信息块及堆栈跟踪
func writeDiagnosticError(w io.Writer, start time.Time, workDir, command string, usePty bool, errStr string, exitCode int, stackTrace string) {
	if w == nil {
		return
	}
	modeStr := "Pipe 模式"
	if usePty {
		modeStr = "PTY 伪终端模式"
	}
	detail := errStr
	if exitCode > 0 && errStr != "" && !strings.Contains(errStr, "Exit Code:") {
		detail = fmt.Sprintf("Exit Code: %d, Error: %s", exitCode, errStr)
	} else if exitCode > 0 && errStr == "" {
		detail = fmt.Sprintf("Exit Code: %d", exitCode)
	}

	stackBlock := ""
	if strings.TrimSpace(stackTrace) != "" {
		stackBlock = fmt.Sprintf("\n[Task StackTrace]\n%s", strings.TrimSpace(stackTrace))
	}

	block := fmt.Sprintf(
		"\n================================================================================\n"+
			"[Task Error] 任务执行失败 (%s)%s\n"+
			"[Task Log] 开始时间 : %s\n"+
			"[Task Log] 工作目录 : %s\n"+
			"[Task Log] 运行命令 : %s\n"+
			"[Task Log] 运行模式 : %s\n"+
			"================================================================================\n",
		detail,
		stackBlock,
		start.Format(time.DateTime),
		workDir,
		command,
		modeStr,
	)
	w.Write([]byte(block))
}

// ParseEnvVars 解析环境变量字符串 "KEY1=VALUE1,KEY2=VALUE2"
func ParseEnvVars(envStr string) []string {
	if envStr == "" {
		return nil
	}

	pairs := strings.Split(envStr, ",")
	result := make([]string, 0, len(pairs))

	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		pair = strings.ReplaceAll(pair, "{{COMMA}}", ",")
		pair = strings.ReplaceAll(pair, "{{EQUAL}}", "=")
		pair = strings.ReplaceAll(pair, "{{NL}}", "\n")
		result = append(result, pair)
	}

	return result
}

// FormatEnvVars 将环境变量列表格式化为逗号分隔的字符串
func FormatEnvVars(envs []string) string {
	if len(envs) == 0 {
		return ""
	}

	pairs := make([]string, 0, len(envs))
	for _, pair := range envs {
		idx := strings.Index(pair, "=")
		if idx == -1 {
			continue
		}
		name := pair[:idx]
		value := pair[idx+1:]

		encodedValue := strings.ReplaceAll(value, ",", "{{COMMA}}")
		encodedValue = strings.ReplaceAll(encodedValue, "=", "{{EQUAL}}")
		encodedValue = strings.ReplaceAll(encodedValue, "\n", "{{NL}}")
		pairs = append(pairs, fmt.Sprintf("%s=%s", name, encodedValue))
	}

	return strings.Join(pairs, ",")
}
