package main

import (
	"log"
	"net/url"

	// 使用 pb 作为你生成的协议代码的别名
	pb "github.com/TimeCraker/game-backend-demo/services/auth/proto"
	"github.com/gorilla/websocket"

	// 引用 Google 官方库进行二进制转换
	"google.golang.org/protobuf/proto"
)

func main() {
	// ❗请在这里替换为你登录成功的 Token
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJleHAiOjE3NzI2MDg4ODEsImlhdCI6MTc3MjUyMjQ4MX0.fSHCspqvRCNXSHX3CKeeZsa7YVUJHtfyR3tCi73UyR0"
	u := url.URL{Scheme: "ws", Host: "localhost:8081", Path: "/ws", RawQuery: "token=" + token}

	log.Printf("🚀 正在建立长连接: %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("❌ 连接失败: %v", err)
	}
	defer c.Close()

	// 1. 发送一个移动包测试
	moveMsg := &pb.GameMessage{
		Type:   "move",
		UserId: 1, // 模拟玩家 1
		X:      10.5,
		Y:      0,
		Z:      -5.2,
	}

	data, err := proto.Marshal(moveMsg)
	if err != nil {
		log.Fatalf("❌ 序列化失败: %v", err)
	}

	if err := c.WriteMessage(websocket.BinaryMessage, data); err != nil {
		log.Fatalf("❌ 发送失败: %v", err)
	}
	log.Println("✅ 已发送二进制移动包")

	// 2. 持续接收服务器的回传（聊天历史、位置初始化等）
	// 替换 test_client.go 最后的 for 循环部分
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("⚠️ 停止读取: %v", err)
			return
		}

		resp := &pb.GameMessage{}
		proto.Unmarshal(message, resp)

		switch resp.Type {
		case "chat_history":
			log.Printf("📩 [历史记录] 收到 %d 条消息", len(resp.History))
			for _, h := range resp.History {
				log.Printf("   - %s: %s", h.Sender, h.Content)
			}
		case "init_players":
			log.Printf("📩 [初始位置] 收到 %d 个玩家坐标", len(resp.Players))
		case "move":
			log.Printf("📩 [玩家移动] ID:%d -> 坐标: (%.1f, %.1f, %.1f)", resp.UserId, resp.X, resp.Y, resp.Z)
		case "chat":
			log.Printf("📩 [新聊天] 玩家 %d 说: %s", resp.UserId, resp.Content)
		}
	}
}
