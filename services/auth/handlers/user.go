package handlers

import (
	"net/http"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetMe(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"created":  user.CreatedAt,
		})
	}
}
