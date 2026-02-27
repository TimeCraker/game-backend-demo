package main

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/handlers"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	db  *gorm.DB
	rdb *redis.Client
)

func main() {
	// 1. 连接 MySQL
	dsn := "root:rootpassword@tcp(localhost:3306)/game_dev?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("MySQL连接失败: %v", err)
	}
	// 自动迁移
	db.AutoMigrate(&models.User{})
	log.Println("MySQL connected & migrated")

	// 2. 连接 Redis
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	// 测试 Redis 连接（可省略）

	// 3. 创建 Gin 引擎
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// 注册路由（不需要认证）
	r.POST("/register", handlers.Register(db))

	// 登录路由（不需要认证）
	r.POST("/login", handlers.Login(db))

	// 需要认证的 API 组
	auth := r.Group("/api")
	auth.Use(handlers.AuthMiddleware()) // 稍后实现
	{
		auth.GET("/me", handlers.GetMe(db))
	}

	// 启动服务
	r.Run(":8081")
}
