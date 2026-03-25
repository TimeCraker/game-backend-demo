package handlers

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	// 引入 battle 引擎与 match
	"github.com/TimeCraker/game-backend-demo/services/battle"
	"github.com/TimeCraker/game-backend-demo/services/match"
	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"google.golang.org/protobuf/proto"
)

// Client 代表一个受并发保护的在线玩家连接（兼容大厅与战斗）
type Client struct {
	UserID int
	Conn   *websocket.Conn
	RoomID string
	mu     sync.Mutex // 核心修复：严格保护 WebSocket 的并发写入
}

// WriteMessage 封装并发安全的写入，彻底杜绝 Panic 崩溃
func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

type Room struct {
	ID      string
	Players []int
}

type Hub struct {
	Clients         sync.Map // userID -> *Client (统一管理)
	RegisteredRooms sync.Map
	ActiveBattles   sync.Map // roomID -> *battle.BattleRoom

	roomMu sync.RWMutex
	Rooms  map[string]map[*Client]bool
}

var GlobalHub = &Hub{
	Rooms: make(map[string]map[*Client]bool),
}

func (h *Hub) Register(userID int, client *Client) {
	h.Clients.Store(userID, client)
}

func (h *Hub) Unregister(userID int) {
	h.Clients.Delete(userID)
}

func (h *Hub) Broadcast(message []byte) {
	h.Clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		_ = client.WriteMessage(websocket.BinaryMessage, message)
		return true
	})
}

func (h *Hub) SendToUser(userID int, message []byte) {
	if c, ok := h.Clients.Load(userID); ok {
		client := c.(*Client)
		_ = client.WriteMessage(websocket.BinaryMessage, message)
	}
}

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

func (h *Hub) LeaveRoom(client *Client) {
	if client == nil || client.RoomID == "" {
		return
	}
	roomID := client.RoomID

	// 【核心修复】只要有任何一方掉线或离开，立刻终止房间内的物理引擎，防止后台泄漏
	if roomVal, ok := h.ActiveBattles.Load(roomID); ok {
		if br, castOK := roomVal.(*battle.BattleRoom); castOK {
			br.Stop()
		}
		h.ActiveBattles.Delete(roomID)
		log.Printf("♻️ [Hub] 玩家 %d 离开，对局终止，房间 %s 物理引擎已安全销毁", client.UserID, roomID)
	}

	var remainingClients []*Client
	h.roomMu.Lock()
	if m, ok := h.Rooms[roomID]; ok {
		delete(m, client)
		for c := range m {
			remainingClients = append(remainingClients, c)
		}
		if len(m) == 0 {
			delete(h.Rooms, roomID)
		}
	}
	client.RoomID = ""
	h.roomMu.Unlock()

	// 通知剩下的人对手已离开
	if len(remainingClients) == 0 {
		return
	}

	leaveMsg := &pb.GameMessage{
		Type:   "opponent_left",
		RoomId: roomID,
		UserId: uint32(client.UserID),
	}
	leaveData, err := proto.Marshal(leaveMsg)
	if err != nil {
		return
	}

	for _, c := range remainingClients {
		_ = c.WriteMessage(websocket.BinaryMessage, leaveData)
	}
}

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
		_ = c.WriteMessage(messageType, message)
	}
}

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

			// 实例化服务端权威战斗房间并启动 60Hz 引擎
			battleRoom := battle.NewBattleRoom(matchRes.RoomID, matchRes.Player1, matchRes.Player2)
			h.ActiveBattles.Store(matchRes.RoomID, battleRoom)
			go battleRoom.Start()

			// 将物理引擎产出的快照分发给房间内的所有玩家
			go func(roomID string, r *battle.BattleRoom) {
				for payload := range r.BroadcastCh {
					if len(payload) > 0 {
						GlobalHub.BroadcastToRoom(roomID, websocket.BinaryMessage, payload)
					}
				}
			}(matchRes.RoomID, battleRoom)

			successMsg := &pb.GameMessage{
				Type:   "match_success",
				RoomId: matchRes.RoomID,
			}
			data, err := json.Marshal(successMsg)
			if err == nil {
				if c, ok := h.Clients.Load(int(matchRes.Player1)); ok {
					client := c.(*Client)
					_ = client.WriteMessage(websocket.TextMessage, data)
				}
				if c, ok := h.Clients.Load(int(matchRes.Player2)); ok {
					client := c.(*Client)
					_ = client.WriteMessage(websocket.TextMessage, data)
				}
			}

			log.Printf("🏠 [Hub] 物理房间 %s 已创建，引擎轰鸣启动！(P1:%d, P2:%d)", matchRes.RoomID, matchRes.Player1, matchRes.Player2)
		}
	}()
}
