package handlers

import (
	"fmt"
	"log"
	"net/http"

	// 这里的路径确保和你 go.mod 里的 module 名一致
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Upgrader 负责把普通的 HTTP 请求“升级”成长连接
var upgrader = websocket.Upgrader{
	// 允许跨域（生产环境可以限制域名，开发环境设为 true 方便调试）
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// --- 🟢 第一步：身份大检查 (Authentication) ---

		// 尝试从 URL 后面拿 token (?token=xxx)
		tokenString := c.Query("token")
		if tokenString == "" {
			log.Println("❌ 拒绝连接：没带 Token，你是谁？")
			return
		}

		// 解析 Token，拿不到用户 ID 就直接踢出去
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			log.Printf("❌ 拒绝连接：Token 伪造或过期: %v", err)
			return
		}

		userID := claims.UserID // 成功认领玩家 ID

		// --- 🟡 第二步：协议大升级 (Upgrade) ---

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ 协议升级失败: %v", err)
			return
		}

		// --- 🔵 第三步：登记与清理 (Management) ---

		// 1. 进门登记：让大总管 Hub 把这个连接记在小本本上
		GlobalHub.Register(userID, conn)

		// 2. 离场清理：使用一个 defer 函数包裹所有“身后事”
		// 这样不管是因为报错断开，还是玩家自己关掉，都会执行这里
		defer func() {
			GlobalHub.Unregister(userID) // 从大总管名单里抹除，防止发空信
			conn.Close()                 // 切断物理连接
			log.Printf("👤 玩家 %d 的连接已释放，清理完毕", userID)
		}()

		log.Printf("🔌 玩家 ID:%d 已成功进入游戏大厅连接池", userID)

		// --- 🟠 第四步：消息传送阵 (Message Loop) ---
		// 4. 消息循环
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("⚠️ 玩家 %d 掉线", userID)
				break
			}

			content := string(p)
			log.Printf("📩 收到玩家 %d 消息: %s", userID, content)

			// --- 🟢 【核心功能】持久化：存入数据库 ---
			// 这里的 DB 就是 base.go 里的全局变量
			msgRecord := models.Message{
				Sender:  fmt.Sprintf("玩家 %d", userID),
				Content: content,
			}
			if err := DB.Create(&msgRecord).Error; err != nil {
				log.Printf("❌ 消息存入数据库失败: %v", err)
			}

			// 5. 广播
			broadcastContent := fmt.Sprintf("%s 说: %s", msgRecord.Sender, msgRecord.Content)
			GlobalHub.Broadcast([]byte(broadcastContent))
		}
	}
}
