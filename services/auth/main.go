package main

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/account"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/send_email"
	"github.com/TimeCraker/game-backend-demo/services/auth/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// --- 数据库模块初始化 ---
	db.InitMySQL()
	db.InitRedis()
	// --- 启动 Gin 引擎 ---
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// --- 路由注册 ---

	// 根据你的建议，将基础功能统一放入 v1 组中管理
	v1 := r.Group("/api/v1")
	{
		// 基础账号功能（注册、登录）

		// 更新路由绑定，使用新的包名前缀
		v1.POST("/register", account.Register)
		v1.POST("/login", account.Login)
		// 新增：邮箱验证码接口
		v1.POST("/send-code", send_email.SendEmailCode)

	}

	// 原有的认证组功能
	api := r.Group("/api")

	// 更新中间件和路由绑定
	api.Use(middleware.AuthMiddleware())
	{
		// 只有带 Token 的请求才能访问 /api/me
		api.GET("/me", account.GetMe)
	}

	// WebSocket 入口

	// 更新 WebSocket 路由绑定

	// --- 5. 跑起来！ ---
	log.Println("👾 游戏认证后端已在 :8081 启动")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
