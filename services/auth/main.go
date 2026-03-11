package main

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	// --- 1. 数据库模块初始化 ---
	db.InitMySQL()
	db.InitRedis()

	// --- 2. 核心交接：全局变量赋值 ---
	handlers.DB = db.SQLDB

	// --- 3. 启动 Gin 引擎 ---
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// --- 4. 路由注册 ---

	// ===== 新增代码 START =====
	// 根据你的建议，将基础功能统一放入 v1 组中管理
	v1 := r.Group("/api/v1")
	{
		// 基础账号功能（注册、登录）
		v1.POST("/register", handlers.Register)
		v1.POST("/login", handlers.Login)
		// 新增：邮箱验证码接口
		v1.POST("/send-code", handlers.SendEmailCode)
	}
	// ===== 新增代码 END =====

	// 原有的认证组功能
	api := r.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		// 只有带 Token 的请求才能访问 /api/me
		api.GET("/me", handlers.GetMe)
	}

	// WebSocket 入口
	r.GET("/ws", handlers.HandleWS())

	// --- 5. 跑起来！ ---
	log.Println("👾 游戏认证后端已在 :8081 启动")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
