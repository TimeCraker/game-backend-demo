package handlers

import (
	"encoding/json" // 【更新】引入 JSON 解析库
	"fmt"
	"log"
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 【更新】GameMessage 定义了客户端发来的统一 JSON 指令格式
type GameMessage struct {
	Type    string  `json:"type"`    // 消息类型: "chat" 或 "move"
	Content string  `json:"content"` // 聊天内容
	X       float64 `json:"x"`       // 坐标 X
	Y       float64 `json:"y"`       // 坐标 Y
	Z       float64 `json:"z"`       // 坐标 Z
}

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

		tokenString := c.Query("token")
		if tokenString == "" {
			log.Println("❌ 拒绝连接：没带 Token")
			return
		}

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			log.Printf("❌ 拒绝连接：Token 伪造或过期: %v", err)
			return
		}

		// 统一使用 uint 类型，方便数据库 models 操作
		userID := uint(claims.UserID)

		// --- 🟡 第二步：协议大升级 (Upgrade) ---

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ 协议升级失败: %v", err)
			return
		}

		// --- 🔵 第三步：登记与清理 (Management) ---

		// 1. 进门登记：显式转为 int 适配 Hub 现有的字典类型
		GlobalHub.Register(int(userID), conn)

		// 2. 离场清理：使用一个 defer 函数包裹所有“身后事”
		defer func() {
			GlobalHub.Unregister(int(userID)) // 显式转为 int 适配 Hub
			conn.Close()                      // 切断物理连接
			log.Printf("👤 玩家 %d 的连接已释放，清理完毕", userID)
		}()

		log.Printf("🔌 玩家 ID:%d 已成功进入游戏大厅连接池", userID)

		// --- 🟠 第四步：消息传送阵 (Message Loop) ---
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("⚠️ 玩家 %d 掉线", userID)
				break
			}

			// --- 【更新点】解析 JSON 指令 ---
			var incoming GameMessage
			if err := json.Unmarshal(p, &incoming); err != nil {
				log.Printf("⚠️ 收到非标准格式消息: %s", string(p))
				continue
			}

			// 根据消息类型进行分流处理
			switch incoming.Type {
			case "chat":
				// 处理聊天逻辑
				handleChatLogic(userID, incoming.Content)
			case "move":
				// 处理移动逻辑
				handleMoveLogic(userID, incoming.X, incoming.Y, incoming.Z)
			default:
				log.Printf("❓ 未知指令类型: %s", incoming.Type)
			}
		}
	}
}

// handleChatLogic 处理聊天业务，接收 uint 类型的 userID
func handleChatLogic(userID uint, content string) {
	// 持久化：存入数据库
	msgRecord := models.Message{
		Sender:  fmt.Sprintf("玩家 %d", userID),
		Content: content,
	}
	DB.Create(&msgRecord)

	// 广播消息
	resp := fmt.Sprintf("{\"type\":\"chat\",\"sender\":\"玩家 %d\",\"content\":\"%s\"}", userID, content)
	GlobalHub.Broadcast([]byte(resp))
}

// handleMoveLogic 处理玩家移动业务，接收 uint 类型的 userID
func handleMoveLogic(userID uint, x, y, z float64) {
	// 1. 更新数据库坐标（存在则更新，不存在则创建）
	newPos := models.PlayerPosition{
		UserID: userID,
		X:      x,
		Y:      y,
		Z:      z,
	}
	DB.Where(models.PlayerPosition{UserID: userID}).Assign(newPos).FirstOrCreate(&models.PlayerPosition{})

	// 2. 广播坐标给其他玩家，使用标准 JSON 格式
	moveData := fmt.Sprintf("{\"type\":\"move\",\"user_id\":%d,\"x\":%.2f,\"y\":%.2f,\"z\":%.2f}", userID, x, y, z)
	GlobalHub.Broadcast([]byte(moveData))

	// 【恢复显示】终端现在会打印移动日志了 测试专用 去除注释即可
	//log.Printf("🏃 玩家 %d 移动至 (%.2f, %.2f, %.2f)", userID, x, y, z)
}
