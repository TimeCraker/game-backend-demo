package main

import (
	"log"

	// 【注意】请确保此路径与你的 go.mod 模块名一致
	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/handlers"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 连接 MySQL (此处暂时留在 main，方便你目前 handlers 的调用)
	dsn := "root:rootpassword@tcp(localhost:3306)/game_dev?charset=utf8mb4&parseTime=True&loc=Local"
	mysqlDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ MySQL连接失败: %v", err)
	}
	// 自动迁移用户表
	mysqlDB.AutoMigrate(&models.User{})
	log.Println("✅ MySQL 已连接并完成自动迁移")

	// 2. 连接 Redis (调用你 db/redis.go 里的 InitRedis)
	// 这个函数里我们写了 Ping 测试，如果连不上会直接 panic 报错
	db.InitRedis()
	log.Println("✅ Redis 已通过 db 模块初始化成功")

	// 3. 创建 Gin 引擎
	r := gin.Default()

	// 健康检查接口
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "up",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	// 注册路由
	r.POST("/register", handlers.Register(mysqlDB))
	r.POST("/login", handlers.Login(mysqlDB))

	// 需要认证的 API 组
	api := r.Group("/api")
	api.Use(handlers.AuthMiddleware())
	{
		api.GET("/me", handlers.GetMe(mysqlDB))
	}

	// 4. 启动服务
	log.Println("👾 游戏认证服务器已启动在 :8081")
	if err := r.Run(":8081"); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}
