package router

import (
	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/controllers"
	"github.com/uyloal/baihu-panel/internal/services"
	"github.com/uyloal/baihu-panel/internal/services/tasks"
)

var executorService *tasks.ExecutorService

func RegisterControllers() *Controllers {
	// 初始化服务
	settingsService := services.NewSettingsService()
	loginLogService := services.NewLoginLogService()

	// 执行系统初始化（返回 userService）
	initService := services.NewInitService(settingsService)
	userService := initService.Initialize()

	taskService := tasks.NewTaskService()
	envService := services.NewEnvService()
	scriptService := services.NewScriptService()
	sendStatsService := services.NewSendStatsService()
	systemWSManager := services.GetSystemWSManager()

	taskLogService := tasks.NewTaskLogService(sendStatsService)
	notifyService := services.NewNotificationService()
	appLogService := services.NewAppLogService()

	executorService = tasks.NewExecutorService(taskService, taskLogService, settingsService, envService)
	// 启动时清理残留的运行状态
	_ = executorService.CleanupRunningTasks()

	// 启动计划任务
	executorService.StartCron()

	// 初始化所有关注系统总线的服务
	setupEventHandlers(appLogService, notifyService, loginLogService, systemWSManager)
	startAppLogCleanup(appLogService)

	taskController := controllers.NewTaskController(taskService, executorService)
	envController := controllers.NewEnvController(envService)

	// 初始化并返回控制器
	return &Controllers{
		Task:         taskController,
		Auth:         controllers.NewAuthController(userService, settingsService, loginLogService),
		Env:          envController,
		Script:       controllers.NewScriptController(scriptService),
		Executor:     controllers.NewExecutorController(executorService),
		File:         controllers.NewFileController(constant.ScriptsWorkDir),
		Dashboard:    controllers.NewDashboardController(executorService),
		Log:          controllers.NewLogController(),
		LogSSE:       controllers.NewLogSSEController(),
		Terminal:     controllers.NewTerminalController(envService),
		Settings:     controllers.NewSettingsController(userService, loginLogService, executorService),
		Dependency:   controllers.NewDependencyController(),
		Notification: controllers.NewNotificationController(),
		AppLog:       controllers.NewAppLogController(),
		SystemWS:     controllers.NewSystemWSController(),
		WebUI:        controllers.NewWebUIController(services.NewWebUIService(settingsService)),
		Monitor:      controllers.NewMonitorController(executorService),
		Data:         controllers.NewDataController(taskController, envController),
		Tag:          controllers.NewTagController(services.NewTagService()),
	}
}

// StopCron 停止计划任务服务
func StopCron() {
	if executorService != nil {
		executorService.Stop()
	}
}
