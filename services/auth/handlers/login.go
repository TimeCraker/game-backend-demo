package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	// 引入你写的 db 包 (Redis操作)
	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	// 移除了 gorm.io/gorm，因为直接使用本包 base.go 里的 DB
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

	// 2. 查数据库找用户
	var user models.User
	// 【改动点】使用全局变量 DB 替代 mysqlDB
	if err := DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 3. 验证密码 (Bcrypt 发挥作用的地方)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
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
	// ============================================================

	// 6. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}
