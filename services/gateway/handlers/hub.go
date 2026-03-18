package handlers

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	// 引入匹配包与协议包，用于解析房间与构造成功指令
	"github.com/TimeCraker/game-backend-demo/services/match"
	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"google.golang.org/protobuf/proto"
)

// Client 代表一个在线的玩家连接
type Client struct {
	UserID int
	Conn   *websocket.Conn
}

// 修改内容：定义物理房间结构体
// 修改原因：用于隔离存放同一对局内的玩家 UserID
type Room struct {
	ID      string
	Players []int // 存储房间内所有玩家的 UserID
}

// Hub 是我们的“大总管”结构体
type Hub struct {
	// 存放所有在线连接：key 是用户 ID，value 是对应的连接信息
	// 使用 sync.Map 是为了保证多线程下读写安全（防止多个人同时登录/下线导致程序崩溃）
	Clients sync.Map

	// 存放所有的活跃房间：key 是 RoomID，value 是 *Room
	Rooms sync.Map
}

// GlobalHub 定义一个全局的大总管，方便在任何地方调用
var GlobalHub = &Hub{}

// Register 玩家上线，登记到名单
func (h *Hub) Register(userID int, conn *websocket.Conn) {
	h.Clients.Store(userID, conn)
}

// Unregister 玩家下线，从名单抹除
func (h *Hub) Unregister(userID int) {
	h.Clients.Delete(userID)
	// (可选扩展) 这里如果玩家在房间内，还可以补充将其从房间踢出的逻辑
}

// Broadcast 全服广播：给所有人发消息
func (h *Hub) Broadcast(message []byte) {
	h.Clients.Range(func(key, value interface{}) bool {
		conn := value.(*websocket.Conn)
		// 【关键修改点】将 TextMessage 改为 BinaryMessage
		// 这样客户端接收到数据后，才会尝试按照 Protobuf 格式解析，而不是解析成字符串
		_ = conn.WriteMessage(websocket.BinaryMessage, message)
		return true
	})
}

// 修改内容：新增定向单发与房间物理隔离广播机制
// 修改原因：支持对特定玩家或特定房间内发送数据包，终结无差别全服广播

// SendToUser 精准向某一位玩家发送消息
func (h *Hub) SendToUser(userID int, message []byte) {
	if c, ok := h.Clients.Load(userID); ok {
		conn := c.(*websocket.Conn)
		_ = conn.WriteMessage(websocket.BinaryMessage, message)
	}
}

// BroadcastToRoom 房间内物理隔离广播
func (h *Hub) BroadcastToRoom(roomID string, message []byte) {
	if r, ok := h.Rooms.Load(roomID); ok {
		room := r.(*Room)
		for _, uid := range room.Players {
			h.SendToUser(uid, message)
		}
	}
}

// ListenMatchResults 独立协程：监听匹配引擎搓合结果并进行开房
func (h *Hub) ListenMatchResults() {
	go func() {
		for matchRes := range match.GlobalMatcher.ResultCh {
			room := &Room{
				ID:      matchRes.RoomID,
				Players: []int{int(matchRes.Player1), int(matchRes.Player2)},
			}
			h.Rooms.Store(matchRes.RoomID, room)

			// 构造 match_success 消息通知前端切场景
			successMsg := &pb.GameMessage{
				Type:   "match_success",
				RoomId: matchRes.RoomID,
			}
			// ===== 新增代码 START =====
			// 修改内容：match_success 改为 JSON 文本消息发送给 React 前端
			// 修改原因：大厅(React) 用 TextMessage(JSON)；战斗(Unity) 用 BinaryMessage(Protobuf) 的双端分离架构
			// ===== 新增代码 END =====
			if successMsg.Type == "match_success" {
				data, err := json.Marshal(successMsg)
				if err != nil {
					log.Printf("❌ JSON 序列化失败: %v", err)
				} else {
					if c, ok := h.Clients.Load(int(matchRes.Player1)); ok {
						conn := c.(*websocket.Conn)
						if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
							log.Printf("❌ 发送 match_success(TextMessage) 给玩家 %d 失败: %v", matchRes.Player1, err)
						}
					}
					if c, ok := h.Clients.Load(int(matchRes.Player2)); ok {
						conn := c.(*websocket.Conn)
						if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
							log.Printf("❌ 发送 match_success(TextMessage) 给玩家 %d 失败: %v", matchRes.Player2, err)
						}
					}
				}
			} else {
				payload, err := proto.Marshal(successMsg)
				if err != nil {
					log.Printf("❌ Protobuf 序列化失败: %v", err)
				} else {
					h.SendToUser(int(matchRes.Player1), payload)
					h.SendToUser(int(matchRes.Player2), payload)
				}
			}

			log.Printf("🏠 [Hub] 物理房间 %s 已创建并通知玩家 %d 和 %d！", matchRes.RoomID, matchRes.Player1, matchRes.Player2)
		}
	}()
}
