package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Upgrader 用于将 HTTP 协议升级为 WebSocket 协议
var upgrader = websocket.Upgrader{
	// CheckOrigin 解决跨域问题，现在我们允许所有人连接
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 升级连接
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("❌ 升级 WebSocket 失败: %v", err)
			return
		}
		// 确保函数结束时关闭连接
		defer conn.Close()
		log.Println("🔌 客户端已通过 WebSocket 连接")

		// 2. 循环读写消息 (回音壁逻辑)
		for {
			// 读取客户端发来的消息
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Disconnected: %v", err)
				break
			}

			log.Printf("📩 收到消息: %s", string(p))

			// 原样发回去
			if err := conn.WriteMessage(messageType, p); err != nil {
				log.Printf("Write error: %v", err)
				break
			}
		}
	}
}
