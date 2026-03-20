package handlers

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/TimeCraker/game-backend-demo/services/match"
	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"google.golang.org/protobuf/proto"
)

// Client 代表一个在线的玩家连接（大厅或战斗）
type Client struct {
	UserID int
	Conn   *websocket.Conn
	RoomID string // 非空表示已加入某战斗房间（仅用于 Rooms 映射 bookkeeping）
}

// Room 匹配引擎创建的 room 元数据（允许进入该房间的用户 ID 列表）
type Room struct {
	ID      string
	Players []int
}

// Hub 网关枢纽：大厅连接用 Clients；战斗房间用 Rooms + roomMu
type Hub struct {
	Clients sync.Map // userID -> *websocket.Conn（大厅）

	// RegisteredRooms：匹配结果写入的 room 元数据（校验玩家是否有权进入 roomId）
	RegisteredRooms sync.Map // roomID -> *Room

	roomMu sync.RWMutex
	// Rooms：战斗 WebSocket 客户端按房间隔离（roomID -> 该房间内所有 *Client）
	Rooms map[string]map[*Client]bool
}

// GlobalHub 全局 Hub
var GlobalHub = &Hub{
	Rooms: make(map[string]map[*Client]bool),
}

// Register 大厅玩家上线
func (h *Hub) Register(userID int, conn *websocket.Conn) {
	h.Clients.Store(userID, conn)
}

// Unregister 大厅玩家下线
func (h *Hub) Unregister(userID int) {
	h.Clients.Delete(userID)
}

// Broadcast 全服广播（大厅二进制）
func (h *Hub) Broadcast(message []byte) {
	h.Clients.Range(func(key, value interface{}) bool {
		conn := value.(*websocket.Conn)
		_ = conn.WriteMessage(websocket.BinaryMessage, message)
		return true
	})
}

// SendToUser 单播给指定 user（大厅 map 中的连接）
func (h *Hub) SendToUser(userID int, message []byte) {
	if c, ok := h.Clients.Load(userID); ok {
		conn := c.(*websocket.Conn)
		_ = conn.WriteMessage(websocket.BinaryMessage, message)
	}
}

// JoinRoom 将战斗客户端加入 Rooms[roomID]（须已持有 *Client，Conn 已就绪）
func (h *Hub) JoinRoom(client *Client, roomID string) {
	if client == nil || roomID == "" || client.Conn == nil {
		return
	}
	h.roomMu.Lock()
	defer h.roomMu.Unlock()
	if h.Rooms == nil {
		h.Rooms = make(map[string]map[*Client]bool)
	}
	m, ok := h.Rooms[roomID]
	if !ok {
		m = make(map[*Client]bool)
		h.Rooms[roomID] = m
	}
	client.RoomID = roomID
	m[client] = true
}

// LeaveRoom 从 Rooms 中移除战斗客户端
func (h *Hub) LeaveRoom(client *Client) {
	if client == nil || client.RoomID == "" {
		return
	}
	h.roomMu.Lock()
	defer h.roomMu.Unlock()
	roomID := client.RoomID
	if m, ok := h.Rooms[roomID]; ok {
		delete(m, client)
		if len(m) == 0 {
			delete(h.Rooms, roomID)
		}
	}
	client.RoomID = ""
}

// BroadcastToRoom 向某房间内所有战斗连接广播（保持原始 WebSocket 帧类型）
func (h *Hub) BroadcastToRoom(roomID string, messageType int, message []byte) {
	if roomID == "" {
		return
	}
	h.roomMu.RLock()
	m := h.Rooms[roomID]
	clients := make([]*Client, 0, len(m))
	for c := range m {
		clients = append(clients, c)
	}
	h.roomMu.RUnlock()

	for _, c := range clients {
		if c == nil || c.Conn == nil {
			continue
		}
		_ = c.Conn.WriteMessage(messageType, message)
	}
}

// RoomHasUser 判断用户是否在匹配注册的该房间成员列表中
func (h *Hub) RoomHasUser(roomID string, userID int) bool {
	if r, ok := h.RegisteredRooms.Load(roomID); ok {
		room := r.(*Room)
		for _, uid := range room.Players {
			if uid == userID {
				return true
			}
		}
	}
	return false
}

// ListenMatchResults 监听匹配结果并开房
func (h *Hub) ListenMatchResults() {
	go func() {
		for matchRes := range match.GlobalMatcher.ResultCh {
			room := &Room{
				ID:      matchRes.RoomID,
				Players: []int{int(matchRes.Player1), int(matchRes.Player2)},
			}
			h.RegisteredRooms.Store(matchRes.RoomID, room)

			successMsg := &pb.GameMessage{
				Type:   "match_success",
				RoomId: matchRes.RoomID,
			}
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
