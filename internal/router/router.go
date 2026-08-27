package router

import (
	"os"
	"strings"

	"github.com/uyloal/baihu-panel/internal/controllers"
	"github.com/uyloal/baihu-panel/internal/middleware"
	"github.com/uyloal/baihu-panel/internal/services"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

type Controllers struct {
	Task         *controllers.TaskController
	Auth         *controllers.AuthController
	Env          *controllers.EnvController
	Script       *controllers.ScriptController
	Executor     *controllers.ExecutorController
	File         *controllers.FileController
	Dashboard    *controllers.DashboardController
	Log          *controllers.LogController
	LogSSE       *controllers.LogSSEController
	Terminal     *controllers.TerminalController
	Settings     *controllers.SettingsController
	Dependency   *controllers.DependencyController
	Notification *controllers.NotificationController
	AppLog       *controllers.AppLogController
	SystemWS     *controllers.SystemWSController
	WebUI        *controllers.WebUIController
	Monitor      *controllers.MonitorController
	Data         *controllers.DataController
	Tag          *controllers.TagController
}

func Setup(c *Controllers) *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(middleware.GinLogger(), middleware.GinRecovery())

	// 获取 URL 前缀
	cfg := services.GetConfig()
	urlPrefix := ""
	pprofEnabled := false
	if cfg != nil {
		urlPrefix = strings.TrimSuffix(cfg.Server.URLPrefix, "/")
		pprofEnabled = cfg.Server.PprofEnabled
	}

	// 创建一个路由组，如果有前缀则使用前缀，否则使用根路径
	var root *gin.RouterGroup
	if urlPrefix != "" {
		root = router.Group(urlPrefix)
	} else {
		root = router.Group("")
	}

	// 按需绑定 Pprof 调试路由 (注册在 root 下以支持 URLPrefix)
	if pprofEnabled {
		pprof.RouteRegister(root)
	}

	// =========================================================================
	// 路由分类组装
	// =========================================================================

	// 1. 静态资源路由
	initStaticRoutes(root)

	// 2. 内部 API 路由组
	apiV1 := root.Group("/api/v1")
	initPublicAPIRoutes(apiV1, c)     // 公开接口 (无需认证)
	initAuthorizedAPIRoutes(apiV1, c) // 授权接口 (需 JWT)

	// 3. OpenAPI 路由组
	initOpenAPIV1Routes(root, c)

	// =========================================================================
	// 全局 404 兜底与 SPA 渲染
	// =========================================================================
	router.NoRoute(func(ctx *gin.Context) {
		path := ctx.Request.URL.Path

		if urlPrefix != "" && !strings.HasPrefix(path, urlPrefix) {
			ctx.Status(404)
			return
		}

		relPath := strings.TrimPrefix(path, urlPrefix)
		if !strings.HasPrefix(relPath, "/") {
			relPath = "/" + relPath
		}

		hasAnyExt := false
		if idx := strings.LastIndex(relPath, "."); idx > 0 && len(relPath)-idx < 6 {
			hasAnyExt = true
		}

		if strings.HasPrefix(relPath, "/api/") || strings.HasPrefix(relPath, "/assets/") || strings.HasPrefix(relPath, "/debug/") || hasAnyExt {
			ctx.String(404, "404 Not Found")
			return
		}

		serveSPA(ctx, urlPrefix, 200)
	})

	return router
}
