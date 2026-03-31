package account

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/TimeCraker/game-backend-demo/services/auth/db"
	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/TimeCraker/game-backend-demo/services/auth/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const guestInviteCode = "77"

type guestLoginRequest struct {
	InviteCode string `json:"inviteCode" binding:"required"`
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func randomGuestIdentity() (username string, email string) {
	suffix := strings.ToUpper(randomHex(3))
	if suffix == "" {
		suffix = strings.ToUpper(time.Now().Format("150405"))
	}
	username = "Guest_" + suffix
	email = strings.ToLower("guest_" + suffix + "@guest.asternova.local")
	return
}

// GuestLogin 邀请码游客登录：输入固定邀请码后自动分配随机访客身份并进入大厅
func GuestLogin(c *gin.Context) {
	var req guestLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if strings.TrimSpace(req.InviteCode) != guestInviteCode {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邀请码无效"})
		return
	}

	passwordSeed := randomHex(12)
	if passwordSeed == "" {
		passwordSeed = "guest_fallback_" + time.Now().Format("20060102150405")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordSeed), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	var created models.User
	createdOK := false
	for i := 0; i < 6; i++ {
		username, email := randomGuestIdentity()
		user := models.User{
			Username: username,
			Email:    email,
			Password: string(hashedPassword),
		}
		if err := db.SQLDB.Create(&user).Error; err == nil {
			created = user
			createdOK = true
			break
		}
	}
	if !createdOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "游客身份分配失败，请重试"})
		return
	}

	token, err := utils.GenerateToken(int(created.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token生成失败"})
		return
	}

	_ = db.SetUserOnline(created.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "游客登录成功",
		"token":   token,
		"user": gin.H{
			"id":       created.ID,
			"username": created.Username,
			"email":    created.Email,
			"is_guest": true,
		},
	})
}

