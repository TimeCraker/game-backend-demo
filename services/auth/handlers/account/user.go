package account

import (
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"

	// 引入 db 包以使用 db.SQLDB
	"github.com/TimeCraker/game-backend-demo/services/auth/db"
)

// GetMe 获取当前登录玩家的详细信息
// 【改动点】移除了 (db *gorm.DB) 参数，直接作为 gin.HandlerFunc 使用
func GetMe(c *gin.Context) {
	// 1. 从中间件 Context 中获取 userID (由 AuthMiddleware 解析 Token 后存入)
	userID, exists := c.Get("userID")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
		return
	}

	// 2. 查询数据库
	var user models.User
	// 【关键】直接使用本包 base.go 中定义的全局变量 DB
	if err := db.SQLDB.First(&user, userID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "玩家数据不存在"})
		return
	}

	// 3. 返回脱敏后的用户信息
	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"created":  user.CreatedAt,
	})
}
