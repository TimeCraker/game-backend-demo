package send_email

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
)

// EmailRequest 接收前端发送验证码的请求
type EmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// SendEmailCode 发送验证码接口
func SendEmailCode(c *gin.Context) {
	var req EmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
		return
	}

	email := req.Email
	today := time.Now().Format("2006-01-02")

	// Redis Key 定义
	cooldownKey := "email_cooldown:" + email
	dailyLimitKey := fmt.Sprintf("email_daily_limit:%s:%s", email, today)
	codeKey := "auth_code:" + email

	// ===== 核心逻辑：Redis 限流检查 START =====
	// 1. 60秒冷却检查
	if exists, _ := db.RDB.Exists(db.Ctx, cooldownKey).Result(); exists > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "请稍后再试 (60s 冷却中)"})
		return
	}

	// 2. 每日 5 次限制检查
	dailyCount, _ := db.RDB.Get(db.Ctx, dailyLimitKey).Int()
	if dailyCount >= 5 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "该邮箱今日发送次数已达上限"})
		return
	}
	// ===== 限流检查 END =====

	// 3. 生成 6 位验证码
	rand.Seed(time.Now().UnixNano())
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// 4. 调用真实发送工具
	if err := utils.SendVerificationEmail(email, code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送失败，请检查配置"})
		return
	}

	// 5. 写入 Redis 状态
	db.RDB.Set(db.Ctx, cooldownKey, "1", 60*time.Second)
	db.RDB.Incr(db.Ctx, dailyLimitKey)
	db.RDB.Expire(db.Ctx, dailyLimitKey, 24*time.Hour)
	db.RDB.Set(db.Ctx, codeKey, code, 5*time.Minute)

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送至邮箱"})
}
