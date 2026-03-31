package account

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
)

type loginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type loginWithEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// Login 处理账号密码登录（用户名或邮箱）
func Login(c *gin.Context) {
	var input loginRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	clientIP := c.ClientIP()
	identifier := input.Identifier
	if identifier == "" {
		identifier = clientIP
	}
	failKey := fmt.Sprintf("login_fail_count:%s", identifier)

	if _, err := db.RDB.Get(db.Ctx, failKey).Result(); err == nil {
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

	var user models.User
	if err := db.SQLDB.Where("username = ? OR email = ?", input.Identifier, input.Identifier).First(&user).Error; err != nil {
		_ = db.RDB.Incr(db.Ctx, failKey).Err()
		_ = db.RDB.Expire(db.Ctx, failKey, 5*time.Minute).Err()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		_ = db.RDB.Incr(db.Ctx, failKey).Err()
		_ = db.RDB.Expire(db.Ctx, failKey, 5*time.Minute).Err()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := utils.GenerateToken(int(user.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
		return
	}

	err = db.SetUserOnline(user.ID)
	if err != nil {
		log.Printf("⚠️ 警告：无法将用户 %d 标记为在线: %v", user.ID, err)
	} else {
		log.Printf("👤 玩家 ID:%d 已上线，在线状态已存入 Redis", user.ID)
	}

	_ = db.RDB.Del(db.Ctx, failKey).Err()

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// LoginWithEmail 处理邮箱验证码登录（已注册直接登录，未注册返回 require_setup）
func LoginWithEmail(c *gin.Context) {
	var input loginWithEmailRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	codeKey := "auth_code:" + input.Email
	expectedCode, err := db.RDB.Get(db.Ctx, codeKey).Result()
	if err != nil || expectedCode != input.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误或已过期"})
		return
	}

	var user models.User
	if err := db.SQLDB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusAccepted, gin.H{
				"action":  "require_setup",
				"message": "邮箱验证成功，请设置用户名和密码",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	_ = db.RDB.Del(db.Ctx, codeKey).Err()
	token, err := utils.GenerateToken(int(user.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
		return
	}

	_ = db.SetUserOnline(user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}
