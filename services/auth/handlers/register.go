package handlers

import (
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	// 注意：这里不再需要导入 gorm，因为我们直接用 base.go 里的 DB
)

// RegisterRequest 定义了注册请求的 JSON 格式
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register 处理用户注册逻辑
// 【改动点】现在它直接作为 gin 的 Handler，不需要返回闭包了
func Register(c *gin.Context) {
	var req RegisterRequest

	// 1. 绑定并校验 JSON 输入
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式不正确"})
		return
	}

	// 2. 检查用户名是否已存在
	var existingUser models.User
	// 【改动点】这里直接使用全局变量 DB (来自 base.go)
	if err := DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被占用"})
		return
	}

	// 3. 加密密码 (使用 bcrypt 算法)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误：密码加密失败"})
		return
	}

	// 4. 构造用户模型并存入数据库
	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
	}

	// 【改动点】直接使用全局 DB 进行创建
	if err := DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误：用户保存失败"})
		return
	}

	// 5. 返回成功响应
	c.JSON(http.StatusCreated, gin.H{
		"id":      user.ID,
		"message": "恭喜！玩家账号创建成功",
	})
}
