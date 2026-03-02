📅 游戏后端 Demo 逐日通关计划（1.0版）
第一阶段：账号与基建（Week 1）
这一阶段的目标是：让玩家能注册、登录，且服务器知道“谁在线”。

Day 1：环境闭环（已完成 ✅）

安装 Go, WSL2, Docker。

配置 docker-compose.yml (MySQL, Redis)。

初始化 go mod，建立 services/auth 基础目录。

Day 2：持久化注册（已完成 ✅）

引入 GORM，定义 User 模型。

实现 POST /register：使用 bcrypt 哈希加密存储密码。

实现 POST /login：比对密码并下发 JWT Token。

Day 3：状态感知（今日任务 🚀）

步骤1：编写 internal/common/db/redis.go 初始化连接池。

步骤2：登录成功后，将 user_id 存入 Redis（Key: online_users:{id}, Value: token），设置 24h 过期。

步骤3：实现 GET /api/status/:id 接口，直接查 Redis 返回该用户是否在线。

Day 4：鉴权中间件与用户画像

完善 AuthMiddleware：从 Header 取出 Bearer <token> 并解析。

实现 GET /api/me：通过 Token 解析出的 UID，去数据库查出昵称、等级。

Day 5：代码整改与日志

引入 zap 或 logrus 日志库（别再满屏 fmt.Println 了，面试官不喜欢）。

统一 API 返回格式：{"code": 200, "data": {}, "msg": ""}。

第二阶段：长连接网关（Week 2）
这一阶段的目标是：建立那个“打不断”的 WebSocket 通道。

Day 6：WebSocket 握手

在 services/gateway 下新建服务。

实现 /ws 接口，将 HTTP 协议升级（Upgrade）为 WebSocket。

Day 7：连接池管理 (Manager)

编写 Client 和 ClientManager 结构体。

使用 sync.Map 存储当前所有活跃的 WebSocket 连接。

Day 8：心跳机制（保活）

服务端逻辑：每隔 5 秒向客户端发一个 Ping。

清理逻辑：如果客户端 15 秒没回 Pong，强制断开连接并清理 Redis 在线状态。

Day 9：ProtoBuf 协议引入

安装 protoc 编译器。

定义第一个协议文件 message.proto（包含消息 ID 和 Data 字段）。

用二进制传输代替 JSON，这是游戏后端的门面。

Day 10：网关路由转发

网关收到消息后，根据消息 ID 判断该发给哪个业务模块。

第三阶段：房间与同步（Week 3-4）
这一阶段的目标是：让 A 的移动，能在 B 的屏幕上显示出来。

Day 11-12：房间管理逻辑

实现 CreateRoom 和 JoinRoom。

一个房间对应一个 Goroutine（轻量级线程），负责处理房间内消息。

Day 13-14：服务端权威同步（重点！）

实现 Tick Loop（每秒 20 次循环）。

服务器每 50ms 广播一次房间内所有人的位置快照。

Day 15-20：Unity 客户端接入

在 Unity 中编写 C# 代码连接 WebSocket。

实现最基础的“方块移动同步”。