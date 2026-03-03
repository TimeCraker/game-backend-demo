package main

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	// --- 1. 数据库模块初始化 ---

	// 初始化 MySQL (内部包含连接和 AutoMigrate)
	db.InitMySQL()

	// 初始化 Redis
	db.InitRedis()

	// --- 2. 核心交接：全局变量赋值 ---

	// 【重点】把 db 包里的连接交给 handlers 包的全局变量 DB
	// 这样 websocket.go 和其他 handler 就能直接用 handlers.DB 了
	handlers.DB = db.SQLDB

	// --- 3. 启动 Gin 引擎 ---

	r := gin.Default()

	// 健康检查接口：让运维或 Docker 知道服务状态
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// --- 4. 路由注册 ---

	// 注意：因为使用了全局 handlers.DB，函数后面不需要再传参数了
	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)
	r.GET("/ws", handlers.HandleWS()) // WebSocket 入口

	// --- 5. 需要认证的 API 组 ---

	// 创建一个名为 /api 的组，并挂载身份验证中间件
	api := r.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		// 只要在这个花括号里的接口，都必须带上合法的 Token 才能访问
		api.GET("/me", handlers.GetMe)
	}

	// --- 6. 启动服务 ---

	log.Println("👾 游戏认证服务器已启动在 :8081")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}
