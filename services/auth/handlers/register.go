package handlers

import (
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest 定义了注册请求的 JSON 格式
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// 新增邮箱和验证码字段
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// Register 处理用户注册逻辑
func Register(c *gin.Context) {
	var req RegisterRequest

	// 1. 绑定并校验 JSON 输入
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式不正确"})
		return
	}

	// 1.5 从 Redis 校验验证码 (使用 db.RDB 和 db.Ctx)
	expectedCode, err := db.RDB.Get(db.Ctx, "register_code:"+req.Email).Result()
	if err != nil || expectedCode != req.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误或已过期"})
		return
	}
	// 验证成功后立即删除，防止单码多次注册
	db.RDB.Del(db.Ctx, "register_code:"+req.Email)

	// 2. 检查用户名是否已存在
	var existingUser models.User
	if err := DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被占用"})
		return
	}

	// 2.5 检查邮箱是否已被注册
	if err := DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被注册"})
		return
	}

	// 3. 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	// 4. 构造用户模型并存入数据库
	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
	}

	if err := DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户保存失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      user.ID,
		"message": "恭喜！玩家账号创建成功",
	})
}
