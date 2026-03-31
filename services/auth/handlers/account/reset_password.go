package account

import (
	"net/http"
	"strings"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type resetPasswordRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Code            string `json:"code" binding:"required,len=6"`
	NewPassword     string `json:"newPassword" binding:"required"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
}

func isValidPassword(p string) bool {
	if len(p) < 6 || len(p) > 20 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range p {
		if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			hasLetter = true
			continue
		}
		if ch >= '0' && ch <= '9' {
			hasDigit = true
			continue
		}
		// 仅允许字母和数字
		return false
	}
	return hasLetter && hasDigit
}

// ResetPasswordWithEmail 邮箱验证码重置密码
func ResetPasswordWithEmail(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "两次密码输入不一致"})
		return
	}
	if !isValidPassword(strings.TrimSpace(req.NewPassword)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码需为 6-20 位字母+数字组合"})
		return
	}

	codeKey := "auth_code:" + req.Email
	expectedCode, err := db.RDB.Get(db.Ctx, codeKey).Result()
	if err != nil || expectedCode != req.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误或已过期"})
		return
	}

	var user models.User
	if err := db.SQLDB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "该邮箱未注册"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	if err := db.SQLDB.Model(&user).Update("password", string(hashed)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	_ = db.RDB.Del(db.Ctx, codeKey).Err()

	identifier := user.Username
	if identifier == "" {
		identifier = user.Email
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "密码重置成功，请重新登录",
		"identifier": identifier,
	})
}

