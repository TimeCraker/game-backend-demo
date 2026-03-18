// --- FILE: C:\Users\TimeCraker\Desktop\game-backend-demo\services\auth\main.go ---

package main

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/account"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/send_email"
	"github.com/TimeCraker/game-backend-demo/services/auth/middleware"

	// 引入 gateway 服务的 handlers 包，使用 gw_handlers 别名
	gw_handlers "github.com/TimeCraker/game-backend-demo/services/gateway/handlers"

	// 引入 match 匹配引擎服务
	// 修改内容：新增匹配引擎包导入
	// 修改原因：在主服务中启动独立匹配引擎循环
	"github.com/TimeCraker/game-backend-demo/services/match"

	"github.com/gin-gonic/gin"
)

func main() {
	// --- 数据库模块初始化 ---
	db.InitMySQL()
	db.InitRedis()

	// --- 启动 Gin 引擎 ---
	r := gin.Default()


	// 修改内容：增加全局 CORS 跨域中间件
	// 修改原因：支持前后端分离架构下，React 前端 (localhost:3000) 发起的 OPTIONS 预检请求与 POST 请求
	r.Use(func(c *gin.Context) {
		// 允许你的前端地址跨域（开发环境可以先写 "*"，或者严格写明你的前端地址）
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		// 拦截 OPTIONS 预检请求，直接返回 204 无内容状态码
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// --- 路由注册 ---
	// 将基础功能统一放入 v1 组中管理
	v1 := r.Group("/api/v1")
	{

		// 修改内容：修正路由绑定的 Handler 名称，指向真实存在的函数
		// 修改原因：解决 undefined: send_email.SendCodeHandler / account.RegisterHandler / account.LoginHandler 的编译错误
		v1.POST("/send_code", send_email.SendEmailCode)
		v1.POST("/register", account.Register)
		v1.POST("/login", account.Login)

		// 需要鉴权的路由示例：/api/v1/profile
		v1.GET("/profile", middleware.AuthMiddleware(), func(c *gin.Context) {
			userID, _ := c.Get("userID")
			c.JSON(200, gin.H{
				"message": "获取用户信息成功",
				"user_id": userID,
			})
		})
	}

	// 注入 Gateway 模块的长连接路由 (挂载在 /ws)
	r.GET("/ws", gw_handlers.HandleWS())

	// 修改内容：在此处正式拉起匹配引擎循环与网关监听
	// 修改原因：赋予服务器撮合玩家开房对战的核心驱动力
	log.Println("⚙️ 正在启动独立匹配引擎 Matcher...")
	match.GlobalMatcher.Start()
	log.Println("⚙️ 正在启动网关枢纽监听...")
	gw_handlers.GlobalHub.ListenMatchResults()

	// 启动服务器
	log.Println("🚀 Game Auth Server 启动于 :8081")
	r.Run(":8081")
}