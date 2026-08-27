package middleware

import (
	"strings"

	"github.com/uyloal/baihu-panel/internal/constant"
	"github.com/uyloal/baihu-panel/internal/services"
	"github.com/uyloal/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

// NotifyTokenAuth 通知 Token 认证中间件
func NotifyTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		settingsService := services.NewSettingsService()

		// 1. 优先检查 notify-token header
		token := c.GetHeader("notify-token")
		savedToken := settingsService.Get(constant.SectionNotify, constant.KeyNotifyToken)
		if token != "" && savedToken != "" && strings.EqualFold(token, savedToken) {
			c.Next()
			return
		}

		// 2. 检查 OpenAPI Token (Authorization: Bearer <token>)
		if checkOpenapiToken(c, settingsService) {
			return
		}

		// 3. 检查 Cookie 用户登录态
		cookieToken, err := c.Cookie(constant.CookieName)
		if err == nil && cookieToken != "" {
			if _, _, _, parseErr := utils.ParseToken(cookieToken, constant.Secret); parseErr == nil {
				c.Next()
				return
			}
		}

		utils.Unauthorized(c, "缺少或无效的通知 Token / OpenAPI Token")
		c.Abort()
	}
}
