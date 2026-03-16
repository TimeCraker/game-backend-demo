// --- FILE: services/match/matcher.go ---
package match

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ===== 新增代码 START =====
// 修改内容：创建独立的基于 Tick 循环的匹配引擎
// 修改原因：将匹配逻辑与网关长连接彻底物理隔离，对标微服务架构
// 影响范围：独立的业务包，后续向 gateway 的 Hub 输出搓合结果

// MatchResult 匹配成功后生成的结果对象，抛给网关进行开房
type MatchResult struct {
	RoomID  string
	Player1 uint32
	Player2 uint32
}

// Matcher 核心匹配引擎结构体
type Matcher struct {
	mu       sync.Mutex
	queue    []uint32         // 正在排队的玩家 ID 列表 (匹配池)
	ResultCh chan MatchResult // 匹配成功结果的输出管道，交由 Hub 监听
}

// GlobalMatcher 全局单例匹配引擎
var GlobalMatcher = &Matcher{
	queue:    make([]uint32, 0),
	ResultCh: make(chan MatchResult, 100), // 留足缓冲，防止高并发阻塞
}

// AddPlayer 将玩家加入匹配队列
func (m *Matcher) AddPlayer(userID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 防抖机制：检查玩家是否已经在队列中，防止狂点匹配
	for _, id := range m.queue {
		if id == userID {
			return
		}
	}

	m.queue = append(m.queue, userID)
	fmt.Printf("🎯 [Matcher] 玩家 %d 加入匹配队列，当前排队人数: %d\n", userID, len(m.queue))
}

// RemovePlayer 将玩家移出匹配队列 (用于玩家中途取消匹配或突然断线)
func (m *Matcher) RemovePlayer(userID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, id := range m.queue {
		if id == userID {
			// 原地安全切割 slice，抹除该玩家
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			fmt.Printf("🚪 [Matcher] 玩家 %d 取消匹配/离开队列\n", userID)
			break
		}
	}
}

// Start 开启后台搓合协程 (Tick-Based Loop)
func (m *Matcher) Start() {
	ticker := time.NewTicker(1 * time.Second) // 1Hz，每秒执行一次匹配判定扫描

	go func() {
		fmt.Println("⚙️ [Matcher] 独立匹配引擎已启动，正在监听队列...")
		for range ticker.C {
			m.mu.Lock()

			// 极简 1v1 匹配机制：只要池子里满 2 个人，立刻发车！
			// （后期如果要改成 2v2，这里直接改成 >= 4 即可，扩展性极强）
			if len(m.queue) >= 2 {
				p1 := m.queue[0]
				p2 := m.queue[1]
				m.queue = m.queue[2:] // 把这两个人从排队池里剥离

				// 使用 UUID V4 算法生成全球唯一的房间标识符
				roomID := uuid.New().String()

				// 将搓合成功的指令推入管道，交给 Gateway 网关去创建真实房间
				m.ResultCh <- MatchResult{
					RoomID:  roomID,
					Player1: p1,
					Player2: p2,
				}
				fmt.Printf("⚔️ [Matcher] 搓合成功！生成决斗空间: %s (P1: %d, P2: %d)\n", roomID, p1, p2)
			}

			m.mu.Unlock()
		}
	}()
}
