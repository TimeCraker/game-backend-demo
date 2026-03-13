全栈游戏后端开发里程碑（3.0 终极架构版）
第一阶段：核心地基与高维安全 (Completed ✅)
[x] Day 1-2: 环境搭建与持久化。MySQL/Redis 容器化，GORM 接入，Bcrypt 密码加密。

[x] Day 3-4: 会话安全。JWT 鉴权中间件编写，实现 Header 拦截与路由保护 (/api/me)。

[x] Day 5-6: 工业级注册流。整合 SMTP 协议，实现带精美 HTML 模板的验证码邮件投递。

[x] Day 7: 缓存防御。基于 Redis 实现邮箱验证码的 60s 冷却与每日 5 次限流，防止接口被刷。

第二阶段：长连接大厅与协议进化 (Completed ✅)
[x] Day 8-9: WebSocket 握手与心跳。Gorilla 协议升级，建立 GlobalHub，20s Ping / 60s Pong 踢出僵尸连接。

[x] Day 10-11: 二进制协议升级。全面引入 Google Protobuf 替代 JSON，定义 game.proto，网络带宽消耗降低 60%。

[x] Day 12-13: 全服状态广播。实现玩家上线快照同步 (init_players)、移动广播 (move)、离线清理 (logout) 与聊天流回溯。

[x] Day 14-15: Unity 基础接入。C# 异步消息队列处理，Vector3.MoveTowards 平滑插值，3D NameTag 动态渲染。

第三阶段：网关引入与 UI 闭环 (Ongoing 🚀)
[ ] Day 16-17: API Gateway 搭建

新增 Gateway 服务，作为全网唯一入口。

将 HTTP 路由反向代理至内网 Auth 服务，WebSocket 路由代理至 Game 服务进程。

[ ] Day 18-19: Unity 业务面板开发

UGUI 制作带 UI 的登录/注册/验证码收发界面。

UnityWebRequest 接入 HTTP 接口，动态缓存 JWT Token 用于场景切换。

[ ] Day 20: 剥离硬编码与配置化

将 Go 和 Unity 中的 IP、端口、密钥全部抽离为 .env 和 ScriptableObject 配置中心。

第四阶段：跨平台特化与异常处理 (Upcoming 🕒)
[ ] Day 21-22: WebGL 架构特化

引入 NativeWebSocket，通过 #if UNITY_WEBGL 宏定义解决原生网络库在网页端的崩溃问题。

[ ] Day 23: 异常自愈机制

Unity 侧实现断网重连 UI、Token 过期 (401) 自动退回登录页逻辑。

[ ] Day 24: 移动端 Input 适配

引入 Unity Input System，剥离 PC 鼠标点击逻辑，兼容手机虚拟摇杆触控。

第五阶段：部署上线与业务裂变 (Endgame 🏆)
[ ] Day 25-26: 云原生部署

编写集群 docker-compose.yml。配置 Nginx 反向代理与 HTTPS/WSS 证书，上线云服务器。

[ ] Day 27+: 分支裂变与 Match 匹配

预留 Match 匹配队列逻辑。

从主干框架拉出 Git 分支，正式开始研发具体的游戏玩法（如大乱斗、种田、解谜等）。