package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Upgrader 负责把普通的 HTTP 请求“升级”成长连接
var upgrader = websocket.Upgrader{
	// 允许跨域（开发环境设为 true 方便调试）
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// --- 🟢 第一步：身份校验 (Authentication) ---
		tokenString := c.Query("token")
		if tokenString == "" {
			log.Println("❌ 拒绝连接：缺少 Token")
			return
		}

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			log.Printf("❌ 拒绝连接：Token 无效: %v", err)
			return
		}

		userID := claims.UserID

		// --- 🟡 第二步：协议升级 (Upgrade) ---
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ 协议升级失败: %v", err)
			return
		}

		// --- 🔵 第三步：连接管理 (Management) ---
		// 1. 注册连接到全局 Hub
		GlobalHub.Register(userID, conn)

		// 2. 离场清理：使用 defer 确保连接断开时回收资源
		defer func() {
			GlobalHub.Unregister(userID)
			conn.Close()
			log.Printf("👤 玩家 %d 连接释放", userID)
		}()

		log.Printf("🔌 玩家 ID:%d 已进入游戏大厅", userID)

		// --- 🟠 第四步：消息处理循环 (Message Loop) ---
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("⚠️ 玩家 %d 掉线", userID)
				break
			}

			content := string(p)
			log.Printf("📩 收到玩家 %d 消息: %s", userID, content)

			// --- 🟢 【核心功能】消息持久化：存入 MySQL ---
			// 直接使用 handlers 包内的全局变量 DB (来自 base.go)
			msgRecord := models.Message{
				Sender:  fmt.Sprintf("玩家 %d", userID),
				Content: content,
			}

			// 执行入库操作
			if err := DB.Create(&msgRecord).Error; err != nil {
				log.Printf("❌ 消息存入数据库失败: %v", err)
			} else {
				log.Printf("💾 消息已成功持久化，ID: %d", msgRecord.ID)
			}

			// --- 🔵 第五步：实时广播 ---
			broadcastContent := fmt.Sprintf("%s 说: %s", msgRecord.Sender, msgRecord.Content)
			GlobalHub.Broadcast([]byte(broadcastContent))
		}
	}
}
