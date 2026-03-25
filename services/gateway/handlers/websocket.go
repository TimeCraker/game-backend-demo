package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"

	"github.com/TimeCraker/game-backend-demo/services/battle"
	"github.com/TimeCraker/game-backend-demo/services/match"
	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const (
	pingPeriod = 20 * time.Second
	pongWait   = 60 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Query("token")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
			return
		}

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 token"})
			return
		}

		userID := int(claims.UserID)
		roomID := c.Query("roomId")
		if roomID == "" {
			roomID = c.Query("room_id")
		}

		scope := c.Query("scope")
		if scope == "" {
			if roomID != "" {
				scope = "battle"
			} else {
				scope = "lobby"
			}
		}

		if scope == "battle" && roomID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "battle scope requires roomId"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		// 【核心修复】无论是大厅还是战斗，全部包装为带并发锁的安全 Client
		client := &Client{UserID: userID, Conn: conn}

		if scope == "lobby" {
			GlobalHub.Register(userID, client)
			sendInitialPlayersData(client)
			broadcastNewPlayerJoin(userID)
		} else {
			if !GlobalHub.RoomHasUser(roomID, userID) {
				_ = client.WriteMessage(websocket.TextMessage, []byte(`{"error":"非法房间"}`))
				_ = conn.Close()
				return
			}
			GlobalHub.JoinRoom(client, roomID)
			log.Printf("🏠 玩家 %d 已加入战斗房间 roomId=%s", userID, roomID)
		}

		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		// 心跳协程：现在通过 client.WriteMessage 发送，完美杜绝了多协程并发写入 Panic
		go func() {
			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()
			for {
				<-ticker.C
				if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}()

		// 唯一的、干净的读消息循环
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				match.GlobalMatcher.RemovePlayer(uint32(userID))
				if scope == "battle" {
					GlobalHub.LeaveRoom(client)
				} else {
					GlobalHub.Unregister(userID)
					broadcastPlayerLeave(userID)
				}
				break
			}

			// 统一解码层
			var msg pb.GameMessage
			if messageType == websocket.TextMessage {
				if err := json.Unmarshal(message, &msg); err != nil {
					continue
				}
			} else if messageType == websocket.BinaryMessage {
				if err := proto.Unmarshal(message, &msg); err != nil {
					continue
				}
			} else {
				continue
			}

			// 全局拦截：匹配请求
			if msg.Type == "match_req" {
				match.GlobalMatcher.AddPlayer(uint32(userID))
				continue
			}

			// 战斗作用域路由
			if scope == "battle" {
				if msg.Type == "input" {
					if roomValue, ok := GlobalHub.ActiveBattles.Load(roomID); ok {
						if battleRoom, castOK := roomValue.(*battle.BattleRoom); castOK {
							// 非阻塞丢入物理引擎，绝不卡死主协程
							select {
							case battleRoom.InputCh <- battle.InputEvent{
								UserID: uint32(userID),
								Input: battle.InputSnapshot{
									InputX:      float64(msg.InputX),
									InputY:      float64(msg.InputY),
									IsCharging:  msg.IsCharging,
									IsAttacking: msg.IsAttacking,
									MouseX:      float64(msg.MouseX),
									MouseY:      float64(msg.MouseY),
								},
							}:
							default:
							}
						}
					}
				}
				continue
			}

			// 大厅作用域路由
			if msg.Type == "chat" {
				handleChatLogic(userID, msg.Content)
			} else if msg.Type == "move" {
				handleMoveLogic(userID, msg.X, msg.Y, msg.Z)
			}
		}
	}
}

func sendInitialPlayersData(client *Client) {
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
		data := &pb.GameMessage{Type: "init_players", Players: pbPlayers}
		payload, _ := proto.Marshal(data)
		_ = client.WriteMessage(websocket.BinaryMessage, payload)
	}
}

func handleChatLogic(userID int, content string) {
	msgRecord := models.Message{Sender: fmt.Sprintf("玩家 %d", userID), Content: content}
	db.SQLDB.Create(&msgRecord)
	resp := &pb.GameMessage{Type: "chat", Content: content, UserId: uint32(userID)}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}

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
	resp := &pb.GameMessage{Type: "move", UserId: uint32(userID), X: x, Y: y, Z: z}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}

func broadcastNewPlayerJoin(userID int) {
	var pos models.PlayerPosition
	if err := db.SQLDB.Where("user_id = ?", userID).First(&pos).Error; err == nil {
		resp := &pb.GameMessage{
			Type: "init_players",
			Players: []*pb.PlayerPos{
				{UserId: uint32(userID), X: float32(pos.X), Y: float32(pos.Y), Z: float32(pos.Z), RotY: 0},
			},
		}
		payload, _ := proto.Marshal(resp)
		GlobalHub.Broadcast(payload)
	}
}

func broadcastPlayerLeave(userID int) {
	resp := &pb.GameMessage{Type: "logout", UserId: uint32(userID)}
	payload, _ := proto.Marshal(resp)
	GlobalHub.Broadcast(payload)
}
