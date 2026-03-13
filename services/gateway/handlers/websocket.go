package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	// 更新 proto 导入路径，并引入 db 包
	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"google.golang.org/protobuf/proto"
)

// 心跳配置常量：用于稳定性保障，防止僵尸连接
const (
	pingPeriod = 20 * time.Second
	pongWait   = 60 * time.Second
)

// Upgrader 负责把普通的 HTTP 请求“升级”成长连接
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// --- 🟢 第一步：身份大检查 (Authentication) ---
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

		userID := uint(claims.UserID)

		// --- 🟡 第二步：协议升级 (Upgrade) ---
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ 协议升级失败: %v", err)
			return
		}

		// --- 🔵 第三步：连接管理 (Management) ---
		GlobalHub.Register(int(userID), conn)

		// 同步最近 10 条聊天记录（聊天回溯）
		syncChatHistory(conn)

		// 同步当前在线的所有玩家位置给这个刚进入的玩家（功能 A）
		syncWorldState(conn, userID)

		// 稳定性保障：配置心跳感应与读取限制
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		// 启动心跳协程
		go func(conn *websocket.Conn) {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			for range ticker.C {
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}(conn)

		// 2. 离场清理
		defer func() {
			GlobalHub.Unregister(int(userID))
			conn.Close()

			// 【更新功能：Protobuf】使用二进制格式广播下线
			leaveMsg := &pb.GameMessage{
				Type:   "logout",
				UserId: uint32(userID),
			}
			payload, _ := proto.Marshal(leaveMsg)
			GlobalHub.Broadcast(payload)

			log.Printf("👤 玩家 %d 的连接已释放，已广播离开消息并清理资源", userID)
		}()

		log.Printf("🔌 玩家 ID:%d 已成功进入游戏大厅，[Protobuf 模式启动]", userID)

		// --- 🟠 第四步：消息传送阵 (Message Loop) ---
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("⚠️ 玩家 %d 掉线或连接超时", userID)
				break
			}

			// --- 【Protobuf】解析二进制指令 ---
			incoming := &pb.GameMessage{}
			if err := proto.Unmarshal(p, incoming); err != nil {
				log.Printf("⚠️ 收到损坏的二进制包: %v", err)
				continue
			}

			// 根据消息类型进行分流处理
			switch incoming.Type {
			case "chat":
				handleChatLogic(userID, incoming.Content)
			case "move":
				handleMoveLogic(userID, float64(incoming.X), float64(incoming.Y), float64(incoming.Z))
			default:
				log.Printf("❓ 未知二进制指令类型: %s", incoming.Type)
			}
		}
	}
}

// 【Protobuf】同步聊天历史
func syncChatHistory(conn *websocket.Conn) {
	var msgs []models.Message
	db.SQLDB.Order("id desc").Limit(10).Find(&msgs)

	// 将数据库模型转换为 Protobuf 列表
	var pbHistory []*pb.ChatLog
	for _, m := range msgs {
		pbHistory = append(pbHistory, &pb.ChatLog{
			Sender:  m.Sender,
			Content: m.Content,
		})
	}

	data := &pb.GameMessage{
		Type:    "chat_history",
		History: pbHistory,
	}

	payload, _ := proto.Marshal(data)
	// 【更新】使用 BinaryMessage 发送二进制流
	_ = conn.WriteMessage(websocket.BinaryMessage, payload)
}

// 【更新功能：Protobuf】同步世界位置快照
// 【修复Bug】修改查询逻辑，现在只同步当前 GlobalHub 中实际在线的玩家，过滤掉已下线的“幽灵”
func syncWorldState(conn *websocket.Conn, currentUserID uint) {
	var pbPlayers []*pb.PlayerPos

	// 遍历大总管名单，只找当前在线的玩家
	GlobalHub.Clients.Range(func(key, value interface{}) bool {
		onlineUserID := uint(key.(int))

		// 排除掉自己，只同步别人的位置
		if onlineUserID != currentUserID {
			var pos models.PlayerPosition
			// 去数据库里查这个在线玩家的最新坐标
			if err := db.SQLDB.Where("user_id = ?", onlineUserID).First(&pos).Error; err == nil {
				pbPlayers = append(pbPlayers, &pb.PlayerPos{
					UserId: uint32(pos.UserID),
					X:      float32(pos.X),
					Y:      float32(pos.Y),
					Z:      float32(pos.Z),
				})
			}
		}
		return true
	})

	// 只有当存在其他在线玩家时，才发送初始化包
	if len(pbPlayers) > 0 {
		data := &pb.GameMessage{
			Type:    "init_players",
			Players: pbPlayers,
		}

		payload, _ := proto.Marshal(data)
		_ = conn.WriteMessage(websocket.BinaryMessage, payload)
	}
}

// 【Protobuf】处理聊天业务
func handleChatLogic(userID uint, content string) {
	msgRecord := models.Message{
		Sender:  fmt.Sprintf("玩家 %d", userID),
		Content: content,
	}
	db.SQLDB.Create(&msgRecord)

	// 构造 Protobuf 响应包
	resp := &pb.GameMessage{
		Type:    "chat",
		Content: content,
		UserId:  uint32(userID),
	}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}

// 【Protobuf】处理玩家移动业务
func handleMoveLogic(userID uint, x, y, z float64) {
	newPos := models.PlayerPosition{
		UserID: userID,
		X:      x,
		Y:      y,
		Z:      z,
	}
	db.SQLDB.Where(models.PlayerPosition{UserID: userID}).Assign(newPos).FirstOrCreate(&models.PlayerPosition{})

	// 构造二进制广播包
	moveData := &pb.GameMessage{
		Type:   "move",
		UserId: uint32(userID),
		X:      float32(x),
		Y:      float32(y),
		Z:      float32(z),
	}
	payload, _ := proto.Marshal(moveData)
	GlobalHub.Broadcast(payload)

	// 保留测试用注释：
	// log.Printf("🏃 玩家 %d 移动至 (%.2f, %.2f, %.2f)", userID, x, y, z)
}
