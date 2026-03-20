package account

import (
	"fmt"
	"log"
	"net/http"
	"time" // 修复报错：引入 time 包

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
)

// Login 处理用户登录逻辑
// 【改动点】去掉了 mysqlDB 参数，现在它直接就是一个标准的 gin Handler
func Login(c *gin.Context) {
	// 1. 获取登录参数
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// ===== 新增代码 START =====
	// 登录防爆破：基于 Redis 的失败次数计数与封禁
	clientIP := c.ClientIP()
	identifier := input.Username
	if identifier == "" {
		identifier = clientIP
	}
	failKey := fmt.Sprintf("login_fail_count:%s", identifier)

	// 先检查当前是否已被封禁
	// 修复报错：将声明但未使用的 countStr 改为 _
	if _, err := db.RDB.Get(db.Ctx, failKey).Result(); err == nil {
		// 已存在计数时进行阈值判断
		// 不严格区分解析错误，失败时默认跳过，避免因为 Redis 问题影响正常登录
		var current int64
		if n, parseErr := db.RDB.Get(db.Ctx, failKey).Int64(); parseErr == nil {
			current = n
		}

		if current >= 20 {
			_ = db.RDB.Expire(db.Ctx, failKey, 24*time.Hour).Err()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "恶意请求，封禁 24 小时"})
			return
		}
		if current >= 10 {
			_ = db.RDB.Expire(db.Ctx, failKey, 5*time.Minute).Err()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "尝试次数过多，请 5 分钟后再试"})
			return
		}
	}
	// ===== 新增代码 END =====

	// 2. 查数据库找用户
	var user models.User
	// 【改动点】使用全局变量 DB 替代 mysqlDB

	// 替换失效的 DB 为 db.SQLDB
	if err := db.SQLDB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		// ===== 新增代码 START =====
		// 用户名不存在也视为一次失败尝试，增加计数
		_ = db.RDB.Incr(db.Ctx, failKey).Err()
		// 对新创建的计数设置基础过期时间，避免长期占用
		_ = db.RDB.Expire(db.Ctx, failKey, 5*time.Minute).Err()
		// ===== 新增代码 END =====
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 3. 验证密码 (Bcrypt 发挥作用的地方)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		// ===== 新增代码 START =====
		// 密码错误，同样累加失败计数并设置过期
		_ = db.RDB.Incr(db.Ctx, failKey).Err()
		_ = db.RDB.Expire(db.Ctx, failKey, 5*time.Minute).Err()
		// ===== 新增代码 END =====
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 4. 生成 JWT Token (颁发 VIP 胸牌)
	// 🟢 这里会自动调用上面定义的 72 小时逻辑
	token, err := utils.GenerateToken(int(user.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
		return
	}

	// ================== 【核心：Redis 操作】 ==================
	// 5. 将用户标记为在线
	err = db.SetUserOnline(user.ID)
	if err != nil {
		log.Printf("⚠️ 警告：无法将用户 %d 标记为在线: %v", user.ID, err)
	} else {
		log.Printf("👤 玩家 ID:%d 已上线，在线状态已存入 Redis", user.ID)
	}

	// ===== 新增代码 START =====
	// 登录成功后清空失败计数，防止历史失败影响后续正常登录
	_ = db.RDB.Del(db.Ctx, failKey).Err()
	// ===== 新增代码 END =====
	// ============================================================

	// 6. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}
