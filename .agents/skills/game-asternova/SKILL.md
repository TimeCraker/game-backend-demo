---
name: game-asternova
description: "AsterNova 游戏项目运维 - 部署、状态、日志、故障排查。Next.js 前端 + Go(Gin) 后端，阿里云 :3001/:8081，game.asterforge.top / api.asterforge.top。含 CynosDB MySQL、本地交叉编译部署（禁服务器编译）。"
---

# game-asternova

AsterNova 游戏项目全栈运维：前后端部署、服务器维护、故障排查。Git 是 source of truth。阿里云内存仅 1.6GB，**铁律：禁止在服务器上编译**。

## 架构速查

```
用户浏览器
    │
    ▼  HTTPS（DNS -> 阿里云 8.162.7.172，有 ICP 备案）
┌─ 阿里云 Nginx (SSL 终端) ─────────────────────────────────┐
│  game.asterforge.top ──-> 127.0.0.1:3001 (AsterNova 前端)  │
│  api.asterforge.top  ──-> 127.0.0.1:8081 (AsterNova 后端)  │
└───────────────────────────────────────────────────────────┘
                │
    MySQL (CynosDB): sh-cynosdbmysql-grp-8hg8mbfg.sql.tencentcdb.com:29155
    Redis: localhost:6380 (阿里云 Docker)
```

## 关键信息速查

| 项目 | 值 |
|------|-----|
| 前端仓库 | TimeCraker/asternova-web-client（Next.js）|
| 后端仓库 | TimeCraker/game-backend-demo（Go/Gin）|
| 部署手册 | TimeCraker/asterforge-deploy |
| 主服务器 | root@8.162.7.172（阿里云，**1.6GB 内存极度紧张**）|
| SSH 密钥 | `~/Desktop/my_workspace/asterforge-deploy/ssh-keys/aliyun-ecs-login.pem` |
| 前端路径 | /home/admin/asternova-web-client |
| 后端路径 | /home/admin/game-backend-demo |
| 前端服务 | asternova-web（:3001）|
| 后端服务 | asternova-server（:8081）|
| MySQL | CynosDB `sh-cynosdbmysql-grp-8hg8mbfg.sql.tencentcdb.com:29155`，库 game_dev |
| Redis | localhost:6380（阿里云 Docker）|

## 服务管理

```bash
# 状态 / 重启 / 日志
sudo systemctl status|restart asternova-server  # 后端
sudo systemctl status|restart asternova-web     # 前端
sudo journalctl -u <服务名> --no-pager -n 50

# 健康检查
curl -s http://127.0.0.1:8081/health   # 期望: {"mysql":"connected","redis":"connected","status":"up"}
curl -s -o /dev/null -w '%{http_code}' https://game.asterforge.top          # 期望: 200
curl -s -o /dev/null -w '%{http_code}' https://api.asterforge.top/health    # 期望: 200
```

## 部署流程

### 前端 (Next.js)

```bash
# 1. SSH 到服务器，cd /home/admin/asternova-web-client
# 2. git pull origin main + 改 .env.production（如有变更）
# 3. 释放内存（停 RAG Docker）
docker stop rag-demo-lobechat rag-demo-proxy
# 4. 构建（限制内存 512MB）
NODE_OPTIONS='--max-old-space-size=512' npx next build
# 5. 重启 + 恢复 RAG
sudo systemctl restart asternova-web
docker start rag-demo-lobechat rag-demo-proxy
```

### 后端 (Go) · ⚠️ 本地交叉编译

**铁律：禁止在阿里云上编译！1.6GB 内存会 OOM 崩溃。必须本地交叉编译后上传。**

```powershell
# 1. 本地 cd game-backend-demo
# 2. 交叉编译（需要 Go 环境）
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -buildvcs=false -o asternova-server .

# 3. 上传 + 服务器重启
scp -F /dev/null -i "<密钥>" asternova-server root@8.162.7.172:/home/admin/game-backend-demo/asternova-server
ssh -F /dev/null -i "<密钥>" root@8.162.7.172 'chmod +x /home/admin/game-backend-demo/asternova-server && systemctl restart asternova-server'
```

## 关键配置

### 后端 .env (`/home/admin/game-backend-demo/.env`)
```
DATABASE_DSN="game_user:密码@tcp(sh-cynosdbmysql-grp-8hg8mbfg.sql.tencentcdb.com:29155)/game_dev?charset=utf8mb4&parseTime=True&loc=Local"
```
- MySQL：`services/auth/db/mysql.go` 从 `DATABASE_DSN` 读取
- Redis：`services/auth/db/redis.go` **硬编码** `localhost:6380`（端口变了要改代码重编译）
- 端口：8081（main.go）

### 前端 .env.production
```
NEXT_PUBLIC_API_URL=https://api.asterforge.top
NEXT_PUBLIC_WS_URL=wss://api.asterforge.top/ws
```

### Nginx (`/etc/nginx/sites-available/aliyun-proxy.conf`)
- `game.asterforge.top` -> 127.0.0.1:3001
- `api.asterforge.top` -> 127.0.0.1:8081（含 WebSocket upgrade）

### CynosDB 用户
- `game_user@8.162.7.172` - 游戏后端专用
- `root` - 管理员（重置密码等）

## 故障排查

| 现象 | 原因 | 处理 |
|------|------|------|
| 登录 500 broken pipe | MySQL 连接过期 | `systemctl restart asternova-server` |
| 登录 access denied | game_user 密码失效 | 用 root 连 CynosDB 执行 `ALTER USER 'game_user'@'8.162.7.172' IDENTIFIED BY '新密码'` |
| Redis panic | Redis 端口/地址不对 | 检查 `redis.go` Addr 是否 `localhost:6380` |
| 匹配失败"游戏链接失败" | WS URL 路径错误 | 检查 `.env.production` 的 `NEXT_PUBLIC_WS_URL` 含 `/ws` 后缀 |
| 后端启动后 3306 连不上 | .env 的 DATABASE_DSN 没被读取 | 检查 `mysql.go` 用 `os.Getenv("DATABASE_DSN")` |
| 服务器 OOM 崩溃 | 在 1.6GB 机器上编译了 | 控制台重启；**禁止在服务器 apt install / go build / npm build 大项目** |
| SSL 证书过期 | certbot 到期 | `sudo certbot renew && sudo systemctl reload nginx` |
| Nginx 502 | 后端没启动 | `systemctl status asternova-server` |

## 已知风险

1. **阿里云内存仅 1.6GB**：RAG LobeChat 占 561MB，不演示时停掉
2. **Redis 地址硬编码**：`redis.go` 中 `localhost:6380`，Docker Redis 端口变了要改代码重编译
3. **MySQL 连接池无 keepalive**：长时间无请求后连接 stale，重启后端恢复
4. **CynosDB 白名单**：阿里云 IP 变了需在控制台加白名单

## 完成标准

deploy 成功判据：
- `systemctl is-active asternova-server asternova-web` 均为 active
- `curl http://127.0.0.1:8081/health` 返回 `{"status":"up"}`
- `curl https://game.asterforge.top` 200

## 自动进化

会话中执行过非本 skill 的任务后，再次调用时进入进化模式。检测以下信号并更新对应段落：

| 信号 | 更新段落 | 动作 |
|------|---------|------|
| 服务端口/路径/密钥变化 | 关键信息速查 | 改表 |
| 新增故障 + 排查过程 | 故障排查 | 追加一行 |
| Nginx 路由变化 | 关键配置 -> Nginx | 改路由 |
| .env / DATABASE_DSN 变更 | 关键配置 -> .env | 改（不写密码值）|
| CynosDB 白名单/IP 变化 | 已知风险 | 补 |
| Go/Next 编译新坑 | 部署流程 / 故障排查 | 补 |

进化只改本 SKILL.md（单一真相源）。历史交给 git log，不建 DEVLOG。触发判断：会话执行过非本命令任务 -> 进化；只用过本命令 -> 不进化。
