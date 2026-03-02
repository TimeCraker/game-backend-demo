package handlers

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client 代表一个在线的玩家连接
type Client struct {
	UserID int
	Conn   *websocket.Conn
}

// Hub 是我们的“大总管”结构体
type Hub struct {
	// 存放所有在线连接：key 是用户 ID，value 是对应的连接信息
	// 使用 sync.Map 是为了保证多线程下读写安全（防止多个人同时登录/下线导致程序崩溃）
	Clients sync.Map
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
}

// Broadcast 全服广播：给所有人发消息
func (h *Hub) Broadcast(message []byte) {
	h.Clients.Range(func(key, value interface{}) bool {
		conn := value.(*websocket.Conn)
		// 给每个连接发消息
		_ = conn.WriteMessage(websocket.TextMessage, message)
		return true // 继续迭代下一个
	})
}
