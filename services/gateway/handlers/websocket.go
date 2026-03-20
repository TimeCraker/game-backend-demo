package handlers

import (
	"encoding/json"
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

	// 引入匹配引擎包
	"github.com/TimeCraker/game-backend-demo/services/match"
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
			log.Println("❌ 拒绝连接：缺少 token")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
			return
		}

		// 解析 Token 获取当前玩家 ID
		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			log.Println("❌ 拒绝连接：无效的 token ->", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 token"})
			return
		}

		// 显式转换为 int，确保全网关类型统一
		userID := int(claims.UserID)
		log.Printf("✅ 玩家 %d 请求建立 WebSocket 连接...", userID)

		// roomId（camelCase）为主，兼容旧版 room_id
		roomID := c.Query("roomId")
		if roomID == "" {
			roomID = c.Query("room_id")
		}

		// scope：用于显式区分连接语义（lobby/battle）
		// 方案 A：推荐前端显式传 scope；为兼容旧调用，这里保留在 scope 缺失时基于 roomID 推断。
		scope := c.Query("scope")
		if scope == "" {
			if roomID != "" {
				scope = "battle"
			} else {
				scope = "lobby"
			}
		}

		switch scope {
		case "lobby":
			log.Printf("🧭 玩家 %d 建立大厅 WS（scope=lobby）", userID)
		case "battle":
			if roomID == "" {
				log.Printf("❌ 拒绝连接：battle scope 但缺少 roomId，user=%d", userID)
				c.JSON(http.StatusBadRequest, gin.H{"error": "battle scope requires roomId"})
				return
			}
			log.Printf("🧭 玩家 %d 建立战斗 WS（scope=battle, roomId=%s）", userID, roomID)
		default:
			log.Printf("❌ 拒绝连接：未知 scope=%s，user=%d", scope, userID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope"})
			return
		}

		// --- 🟡 第二步：升级协议为 WebSocket (Upgrade) ---
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("❌ 升级 WebSocket 失败:", err)
			return
		}

		var battleClient *Client

		// --- 🔵 第三步：注册玩家并初始化状态 (Initialization) ---
		// battle 连接不进入 GlobalHub.Clients，避免大厅广播误伤战斗帧
		if scope == "lobby" {
			GlobalHub.Register(userID, conn)
			log.Printf("🎉 玩家 %d 已成功加入 GlobalHub", userID)
			sendInitialPlayersData(conn)
			broadcastNewPlayerJoin(userID)
		} else {
			if !GlobalHub.RoomHasUser(roomID, userID) {
				log.Printf("❌ 玩家 %d 尝试加入非法房间 roomId=%s，已拒绝", userID, roomID)
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"非法房间"}`))
				_ = conn.Close()
				return
			}
			battleClient = &Client{UserID: userID, Conn: conn}
			GlobalHub.JoinRoom(battleClient, roomID)
			log.Printf("🏠 玩家 %d 已加入 Hub.Rooms roomId=%s", userID, roomID)
		}

		// --- 心跳机制 (Ping/Pong) ---
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		go func() {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			for {
				<-ticker.C
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}()

		// --- 🔴 第四步：死循环监听前端消息 (Read Loop) ---
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("⚠️ 玩家 %d 断开连接 (roomId=%s): %v", userID, roomID, err)

				// 如果意外断线，顺手从匹配池中移除该玩家，防挂机
				match.GlobalMatcher.RemovePlayer(uint32(userID))

				if battleClient != nil {
					GlobalHub.LeaveRoom(battleClient)
					log.Printf("🏚️ 玩家 %d 已离开 Hub.Rooms roomId=%s", userID, roomID)
				} else {
					GlobalHub.Unregister(userID)
					broadcastPlayerLeave(userID)
				}
				break
			}

			// 方案 A：battle 连接所有消息只转发到房间，不走大厅逻辑
			if scope == "battle" {
				GlobalHub.BroadcastToRoom(roomID, messageType, message)
				continue
			}

			// 解析前端/客户端发来的消息（Text -> JSON；Binary -> Protobuf）
			var msg pb.GameMessage
			switch messageType {
			case websocket.TextMessage:
				if err := json.Unmarshal(message, &msg); err != nil {
					log.Println("❌ JSON 解析失败:", err)
					continue
				}
			case websocket.BinaryMessage:
				if err := proto.Unmarshal(message, &msg); err != nil {
					log.Println("❌ Protobuf 解析失败:", err)
					continue
				}
			default:
				log.Printf("⚠️ 收到不支持的 WebSocket messageType=%d，已忽略", messageType)
				continue
			}

			// 拦截 1：请求匹配
			if msg.Type == "match_req" {
				match.GlobalMatcher.AddPlayer(uint32(userID))
				continue // 处理完毕，阻断后续逻辑
			}

			// --- 如果没有被拦截，说明是大厅全局消息，走传统分流逻辑 ---
			if msg.Type == "chat" {
				handleChatLogic(userID, msg.Content)
			} else if msg.Type == "move" {
				// 仅做为大厅站街时的漫游保存
				handleMoveLogic(userID, msg.X, msg.Y, msg.Z)
			}
		}
	}
}

// 【Protobuf】下发初始化包
func sendInitialPlayersData(conn *websocket.Conn) {
	var pbPlayers []*pb.PlayerPos

	GlobalHub.Clients.Range(func(key, value interface{}) bool {
		id := key.(int)
		var pos models.PlayerPosition
		if err := db.SQLDB.Where("user_id = ?", id).First(&pos).Error; err == nil {
			pbPlayers = append(pbPlayers, &pb.PlayerPos{
				UserId: uint32(pos.UserID),
				X:      float32(pos.X),
				Y:      float32(pos.Y),
				Z:      float32(pos.Z),
				RotY:   0,
			})
		}
		return true
	})

	if len(pbPlayers) > 0 {
		data := &pb.GameMessage{
			Type:    "init_players",
			Players: pbPlayers,
		}

		payload, _ := proto.Marshal(data)
		_ = conn.WriteMessage(websocket.BinaryMessage, payload)
	}
}

// ===== 修改代码 START =====
// 修改内容：统一参数为 userID int，并在使用时强制转换
// 修改原因：修复 cannot use userID (variable of type int) as uint value 的编译报错

// 【Protobuf】处理聊天业务
func handleChatLogic(userID int, content string) {
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
func handleMoveLogic(userID int, x, y, z float32) {
	var pos models.PlayerPosition
	if err := db.SQLDB.Where("user_id = ?", userID).First(&pos).Error; err != nil {
		pos = models.PlayerPosition{UserID: uint(userID), X: float64(x), Y: float64(y), Z: float64(z)}
		db.SQLDB.Create(&pos)
	} else {
		pos.X = float64(x)
		pos.Y = float64(y)
		pos.Z = float64(z)
		db.SQLDB.Save(&pos)
	}

	resp := &pb.GameMessage{
		Type:   "move",
		UserId: uint32(userID),
		X:      x,
		Y:      y,
		Z:      z,
	}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}

// 【Protobuf】处理玩家加入大厅的广播
func broadcastNewPlayerJoin(userID int) {
	var pos models.PlayerPosition
	if err := db.SQLDB.Where("user_id = ?", userID).First(&pos).Error; err == nil {
		resp := &pb.GameMessage{
			Type: "init_players",
			Players: []*pb.PlayerPos{
				{
					UserId: uint32(userID),
					X:      float32(pos.X),
					Y:      float32(pos.Y),
					Z:      float32(pos.Z),
					RotY:   0,
				},
			},
		}
		payload, _ := proto.Marshal(resp)
		GlobalHub.Broadcast(payload)
	}
}

// 【Protobuf】处理玩家离开大厅的广播
func broadcastPlayerLeave(userID int) {
	resp := &pb.GameMessage{
		Type:   "logout",
		UserId: uint32(userID),
	}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}

// ===== 修改代码 END =====
