package db

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

// RDB 全局变量，方便在 handlers 里面直接调用
var RDB *redis.Client

// Go 的 Redis 操作必须带 Context，用于超时控制
var Ctx = context.Background()

func InitRedis() {
	// 初始化 Redis 客户端
	RDB = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 对应 docker-compose 里的映射端口
		Password: "",               // 默认没设密码
		DB:       0,                // 使用 0 号数据库
	})

	// 关键一步：尝试连接并 Ping 一下，确保 Redis 真的活着
	_, err := RDB.Ping(Ctx).Result()
	if err != nil {
		// 如果连不上，直接停掉程序，检查 Docker 是不是没开
		panic(fmt.Sprintf("Redis 连接失败: %v", err))
	}

	fmt.Println("🚀 [Redis] 已就绪，连接成功！")
}

// 辅助函数：将用户标记为在线
func SetUserOnline(userID uint) error {
	key := fmt.Sprintf("online:%d", userID)
	// 存入 Redis，值设为 "1"，有效期 24 小时
	return RDB.Set(Ctx, key, "1", 24*time.Hour).Err()
}