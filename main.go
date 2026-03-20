package main

import (
	"log"
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/account"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers/send_email"
	"github.com/TimeCraker/game-backend-demo/services/auth/middleware"
	// 引入 gateway 服务的 handlers 包，使用 handlers 别名（网关 WebSocket + GlobalHub）
	handlers "github.com/TimeCraker/game-backend-demo/services/gateway/handlers"
	// 引入 match 匹配引擎服务：在主进程中启动 Tick 撮合循环，与网关解耦
	"github.com/TimeCraker/game-backend-demo/services/match"
	"github.com/gin-gonic/gin"
)

func main() {
	// --- 数据库模块初始化 ---
	// 修改内容：在 main 最前初始化 MySQL / Redis
	// 修改原因：后续 account、验证码、会话等均依赖全局 db 客户端
	// 影响范围：进程级，所有 HTTP / WS 处理链路
	db.InitMySQL()
	db.InitRedis()

	// --- 后台守护：匹配引擎 + 网关监听（动态创建物理房间）---
	// 修改内容：拉起 GlobalMatcher（1Hz）与 GlobalHub.ListenMatchResults
	// 修改原因：撮合结果经 ResultCh 送达 Hub，驱动开房与 match_success 下发
	// 影响范围：匹配队列、房间表、WebSocket 推送
	log.Println("⚙️ 正在启动独立匹配引擎 Matcher...")
	match.GlobalMatcher.Start()
	log.Println("⚙️ 正在启动网关枢纽 ListenMatchResults...")
	handlers.GlobalHub.ListenMatchResults()

	// --- 启动 Gin 引擎 ---
	r := gin.Default()

	// ===== 新增代码 START =====
	// 修改内容：全局 CORS 中间件改为允许任意 Origin / Method / Header（通配 *）
	// 修改原因：Next.js 与 Unity WebGL 等多端联调时减少预检失败；与原先「列举常用 Header」等价或更宽
	// 影响范围：所有 HTTP 路由（含 /api、/api/v1、/health）
	r.Use(func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "*")
		h.Set("Access-Control-Allow-Headers", "*")
		h.Set("Access-Control-Expose-Headers", "*")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	// ===== 新增代码 END =====

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// ===== 新增代码 START =====
	// 修改内容：对外统一 REST 前缀 /api（与产品文档一致）
	// 修改原因：与任务要求的 /api/register、/api/login、/api/send-code、/api/me 对齐
	// 影响范围：新接入的客户端可直接走 /api/*
	api := r.Group("/api")
	{
		api.POST("/register", account.Register)
		api.POST("/login", account.Login)
		api.POST("/send-code", send_email.SendEmailCode)

		authz := api.Group("")
		authz.Use(middleware.AuthMiddleware())
		authz.GET("/me", account.GetMe)
	}
	// ===== 新增代码 END =====

	// --- 路由注册：v1 兼容层（现有 Next.js rewrite 仍指向 /api/v1/*）---
	// 修改内容：保留 send_code 路径与 profile 示例接口
	// 修改原因：next.config rewrites：/api/proxy → http://127.0.0.1:8081/api/v1/:path*
	// 影响范围：asternova-web-client 当前登录/注册/验证码请求
	v1 := r.Group("/api/v1")
	{
		// 修正路由绑定的 Handler 名称，指向真实存在的函数
		v1.POST("/send_code", send_email.SendEmailCode)
		v1.POST("/register", account.Register)
		v1.POST("/login", account.Login)

		// 需要鉴权的路由示例：/api/v1/profile（返回结构与 /api/me 不同，保留兼容）
		v1.GET("/profile", middleware.AuthMiddleware(), func(c *gin.Context) {
			userID, _ := c.Get("userID")
			c.JSON(200, gin.H{
				"message": "获取用户信息成功",
				"user_id": userID,
			})
		})
	}

	// 注入 Gateway 模块的长连接路由（大厅 / 战斗统一入口）
	r.GET("/ws", handlers.HandleWS())

	log.Println("🚀 Game Auth + Gateway 启动于 :8081（REST /api、/api/v1，WS /ws）")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}
