package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	// 【关键】引入你刚才写的 db 包，路径要和 go.mod 匹配
	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
)

func Login(mysqlDB *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取登录参数
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		// 2. 查数据库找用户
		var user models.User
		if err := mysqlDB.Where("username = ?", input.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}

		// 3. 验证密码
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}

		// 4. 生成 JWT Token
		token, err := utils.GenerateToken(int(user.ID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
			return
		}

		// ================== 【今日核心：Redis 操作】 ==================
		// 5. 将用户标记为在线
		// 我们调用之前在 db/redis.go 里写好的 SetUserOnline 函数
		err = db.SetUserOnline(user.ID)
		if err != nil {
			// 如果 Redis 挂了，我们记录日志，但不阻止用户登录（这叫“降级处理”）
			log.Printf("⚠️ 警告：无法将用户 %d 标记为在线: %v", user.ID, err)
		} else {
			log.Printf("👤 玩家 ID:%d 已上线，在线状态已存入 Redis", user.ID)
		}
		// ============================================================

		// 6. 返回结果
		c.JSON(http.StatusOK, gin.H{
			"message": "登录成功",
			"token":   token,
		})
	}
}
