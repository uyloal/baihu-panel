package router

import (
	"github.com/uyloal/baihu-panel/internal/middleware"
	"github.com/gin-gonic/gin"
)

func initPublicAPIRoutes(api *gin.RouterGroup, c *Controllers) {
	// Health check (无需认证)
	api.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"message": "pong"})
	})

	// Authentication routes (无需认证)
	auth := api.Group("/auth")
	{
		auth.POST("/login", c.Auth.Login)
		auth.POST("/login/otp", c.Auth.VerifyOTP)
		auth.POST("/logout", c.Auth.Logout)
	}

	// 公开的站点设置（无需认证）
	api.GET("/settings/public", c.Settings.GetPublicSiteSettings)

	// 消息推送发送接口（支持 notify-token、OpenAPI Token 或 登录状态）
	api.POST("/notify/send", middleware.NotifyTokenAuth(), c.Notification.SendNotification)

	// 内部使用的 API（仅限本地调用，无需 Bearer 认证）
	internalAPI := api.Group("/internal")
	internalAPI.Use(middleware.LocalhostOnly())
	{
		internalAPI.POST("/tasks/sync-repo-status", c.Task.SyncRepoTasks)
		internalAPI.POST("/tasks/execute/:id", c.Executor.ExecuteTask)
		internalAPI.POST("/tasks/toggle/:id", c.Task.ToggleTask)
	}
}

func initAuthorizedAPIRoutes(api *gin.RouterGroup, c *Controllers) {
	authorized := api.Group("")
	authorized.Use(middleware.AuthRequired())
	{
		// 获取当前用户 (普通用户即可访问)
		authorized.GET("/auth/me", c.Auth.GetCurrentUser)

		// OTP 两步验证管理 (普通用户即可访问，非 adminOnly)
		otp := authorized.Group("/auth/otp")
		{
			otp.GET("/status", c.Auth.GetOTPStatus)
			otp.POST("/generate", c.Auth.GenerateOTP)
			otp.POST("/enable", c.Auth.EnableOTP)
			otp.POST("/disable", c.Auth.DisableOTP)
		}

		// 以下管理接口需要管理员权限
		adminOnly := authorized.Group("")
		adminOnly.Use(middleware.AdminRequired())
		{
			registerDashboardRoutes(adminOnly, c)
			registerTaskRoutes(adminOnly, c)
			registerEnvRoutes(adminOnly, c)
			registerScriptRoutes(adminOnly, c)
			registerFileRoutes(adminOnly, c)
			registerLogRoutes(adminOnly, c)
			registerTerminalRoutes(adminOnly, c)
			registerSettingsRoutes(adminOnly, c)
			registerDependencyRoutes(adminOnly, c)
			registerNotificationRoutes(adminOnly, c)
			registerAppLogRoutes(adminOnly, c)
			registerSystemWSRoutes(adminOnly, c)
			registerMonitorRoutes(adminOnly, c)
			registerWebUIRoutes(adminOnly, c)
			registerSystemRoutes(adminOnly, c)
			registerTagRoutes(adminOnly, c)
		}
	}
}

func registerDashboardRoutes(g *gin.RouterGroup, c *Controllers) {
	dashboard := g.Group("/dashboard")
	{
		dashboard.GET("/stats", c.Dashboard.GetStats)
		dashboard.GET("/sentence", c.Dashboard.GetSentence)
		dashboard.GET("/sendstats", c.Dashboard.GetSendStats)
		dashboard.GET("/taskstats", c.Dashboard.GetTaskStats)
	}
}

func registerTaskRoutes(g *gin.RouterGroup, c *Controllers) {
	tasks := g.Group("/tasks")
	{
		tasks.GET("", c.Task.List)
		tasks.POST("", c.Task.Create)
		tasks.GET("/tags", c.Task.GetTags)
		tasks.GET("/:id", c.Task.Get)
		tasks.PUT("/:id", c.Task.Update)
		tasks.DELETE("/:id", c.Task.Delete)
		tasks.POST("/batch-delete", c.Task.BatchDelete)
		tasks.DELETE("/batch-by-query", c.Task.BatchDeleteByQuery)
		tasks.POST("/:id/execute", c.Task.Execute)
		tasks.POST("/:id/stop", c.Task.Stop)
		tasks.POST("/:id/toggle", c.Task.Toggle)
		tasks.POST("/batch-toggle", c.Task.BatchToggle)
		tasks.POST("/batch-run", c.Task.BatchRun)
		tasks.POST("/batch-stop", c.Task.BatchStop)
		tasks.POST("/sync-all-repos", c.Task.SyncAllRepos)
		tasks.GET("/:id/logs", c.Task.GetLogs)
		tasks.GET("/:id/last-result", c.Task.GetLastResult)
		tasks.POST("/import", c.Task.ImportTasks)
		tasks.POST("/tags", c.Task.BatchUpdateTags)
		tasks.POST("/pin", c.Task.BatchUpdatePin)
	}
}

func registerEnvRoutes(g *gin.RouterGroup, c *Controllers) {
	env := g.Group("/env")
	{
		env.GET("/secret-status", c.Env.GetSecretStatus)
		env.GET("/tags", c.Env.GetTags)
		env.POST("/bulk-save", c.Env.BulkSaveEnv)
		env.POST("", c.Env.CreateEnvVar)
		env.GET("", c.Env.GetEnvVars)
		env.GET("/all", c.Env.GetAllEnvVars)
		env.GET("/:id", c.Env.GetEnvVar)
		env.GET("/:id/tasks", c.Env.GetAssociatedTasks)
		env.PUT("/:id", c.Env.UpdateEnvVar)
		env.DELETE("/:id", c.Env.DeleteEnvVar)
	}
}

func registerScriptRoutes(g *gin.RouterGroup, c *Controllers) {
	scripts := g.Group("/scripts")
	{
		scripts.POST("", c.Script.CreateScript)
		scripts.GET("", c.Script.GetScripts)
		scripts.GET("/:id", c.Script.GetScript)
		scripts.PUT("/:id", c.Script.UpdateScript)
		scripts.DELETE("/:id", c.Script.DeleteScript)
	}
}

func registerFileRoutes(g *gin.RouterGroup, c *Controllers) {
	files := g.Group("/files")
	{
		files.GET("/tree", c.File.GetFileTree)
		files.GET("/content", c.File.GetFileContent)
		files.GET("/download", c.File.DownloadFile)
		files.GET("/download-zip", c.File.DownloadZip)
		files.POST("/content", c.File.SaveFileContent)
		files.POST("/create", c.File.CreateFile)
		files.POST("/delete", c.File.DeleteFile)
		files.POST("/rename", c.File.RenameFile)
		files.POST("/move", c.File.MoveFile)
		files.POST("/copy", c.File.CopyFile)
		files.POST("/upload", c.File.UploadArchive)
		files.POST("/uploadfiles", c.File.UploadFiles)
	}
}

func registerLogRoutes(g *gin.RouterGroup, c *Controllers) {
	logs := g.Group("/logs")
	{
		logs.GET("", c.Log.GetLogs)
		logs.POST("/clear", c.Log.ClearLogs)
		logs.GET("/sse", c.LogSSE.StreamLog)
		logs.GET("/:id", c.Log.GetLogDetail)
		logs.DELETE("/:id", c.Log.DeleteLog)
	}
}

func registerTerminalRoutes(g *gin.RouterGroup, c *Controllers) {
	g.GET("/terminal/ws", c.Terminal.HandleWebSocket)
	g.GET("/terminal/cmds", c.Terminal.GetCommands)
}

func registerSettingsRoutes(g *gin.RouterGroup, c *Controllers) {
	settings := g.Group("/settings")
	{
		settings.POST("/password", c.Settings.ChangePassword)
		settings.GET("/site", c.Settings.GetSiteSettings)
		settings.PUT("/site", c.Settings.UpdateSiteSettings)
		settings.POST("/site/openapi-token/generate", c.Settings.GenerateOpenapiToken)
		settings.GET("/paths", c.Settings.GetPaths)
		settings.GET("/scheduler", c.Settings.GetSchedulerSettings)
		settings.PUT("/scheduler", c.Settings.UpdateSchedulerSettings)
		settings.GET("/about", c.Settings.GetAbout)
		settings.GET("/changelog", c.Settings.GetChangelog)
		settings.GET("/loginlogs", c.Settings.GetLoginLogs)
		settings.POST("/backup", c.Settings.CreateBackup)
		settings.GET("/backup/status", c.Settings.GetBackupStatus)
		settings.GET("/backup/download", c.Settings.DownloadBackup)
		settings.POST("/restore", c.Settings.RestoreBackup)
		settings.GET("/:section", c.Settings.GetSectionSettings)
		settings.PUT("/:section", c.Settings.UpdateSectionSettings)
		settings.GET("/:section/:key", c.Settings.GetSetting)
		settings.POST("/:section/:key/generate", c.Settings.GenerateSettingToken)
	}
}

func registerDependencyRoutes(g *gin.RouterGroup, c *Controllers) {
	deps := g.Group("/deps")
	{
		deps.GET("", c.Dependency.List)
		deps.POST("", c.Dependency.Create)
		deps.DELETE("/:id", c.Dependency.Delete)
		deps.POST("/install", c.Dependency.Install)
		deps.POST("/install-cmd", c.Dependency.GetInstallCommand)
		deps.POST("/uninstall/:id", c.Dependency.Uninstall)
		deps.POST("/reinstall/:id", c.Dependency.Reinstall)
		deps.POST("/reinstall-all", c.Dependency.ReinstallAll)
		deps.POST("/reinstall-all-cmd", c.Dependency.GetReinstallAllCommand)
		deps.POST("/batch-install-cmd", c.Dependency.GetBatchInstallCommand)
		deps.POST("/import", c.Dependency.ParseAndImport)
		deps.GET("/installed", c.Dependency.GetInstalled)
		deps.GET("/install-suggest-cmd", c.Dependency.GetDepInstallCommand)
	}
}

func registerNotificationRoutes(g *gin.RouterGroup, c *Controllers) {
	notify := g.Group("/notify")
	{
		notify.GET("/types", c.Notification.GetChannelTypes)
		notify.GET("/channels", c.Notification.GetChannels)
		notify.POST("/channels", c.Notification.SaveChannel)
		notify.DELETE("/channels/:id", c.Notification.DeleteChannel)
		notify.POST("/channels/test", c.Notification.TestChannel)
		notify.GET("/bindings", c.Notification.GetBindings)
		notify.POST("/bindings", c.Notification.SaveBinding)
		notify.POST("/bindings/batch", c.Notification.BatchSaveBindings)
		notify.DELETE("/bindings/:id", c.Notification.DeleteBinding)
		notify.GET("/filters", c.Notification.GetFilters)
		notify.POST("/filters", c.Notification.SaveFilter)
		notify.DELETE("/filters/:id", c.Notification.DeleteFilter)
	}
}

func registerAppLogRoutes(g *gin.RouterGroup, c *Controllers) {
	appLogs := g.Group("/app-logs")
	{
		appLogs.GET("", c.AppLog.GetLogs)
		appLogs.POST("/read", c.AppLog.MarkAsRead)
		appLogs.POST("/clear", c.AppLog.ClearLogs)
	}
}

func registerSystemWSRoutes(g *gin.RouterGroup, c *Controllers) {
	g.GET("/ws/events", c.SystemWS.HandleEvents)
}

func registerMonitorRoutes(g *gin.RouterGroup, c *Controllers) {
	monitor := g.Group("/monitor")
	{
		monitor.GET("", c.Monitor.GetSystemMonitor)
		monitor.GET("/sse", c.Monitor.MonitorSSE)
	}
}

func registerWebUIRoutes(g *gin.RouterGroup, c *Controllers) {
	webuiGroup := g.Group("/webui")
	{
		webuiGroup.GET("", c.WebUI.GetWebUIs)
		webuiGroup.POST("/upload", c.WebUI.UploadWebUI)
		webuiGroup.PUT("/active", c.WebUI.SetActiveWebUI)
		webuiGroup.DELETE("/:name", c.WebUI.DeleteWebUI)
	}
}

func registerSystemRoutes(g *gin.RouterGroup, c *Controllers) {
	systemAPI := g.Group("/system")
	{
		systemAPI.POST("/export", c.Data.ExportBusinessData)
		systemAPI.POST("/import", c.Data.ImportBusinessData)
	}
}

func registerTagRoutes(g *gin.RouterGroup, c *Controllers) {
	tags := g.Group("/tags")
	{
		tags.GET("", c.Tag.GetTags)
		tags.POST("", c.Tag.CreateTag)
		tags.PUT("/:id", c.Tag.UpdateTag)
		tags.DELETE("/:id", c.Tag.DeleteTag)
	}
}
