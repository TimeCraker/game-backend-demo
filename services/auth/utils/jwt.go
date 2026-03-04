package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("your-secret-key") // 实际应改为环境变量

type Claims struct {
	// 【修改点】将 uint 改为 int，确保全项目类型统一
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token，有效期改为 72 小时
// 【修改点】参数也改为 int
func GenerateToken(userID int) (string, error) {
	claims := Claims{
		UserID: userID, // 显式指定字段名更清晰
		RegisteredClaims: jwt.RegisteredClaims{
			// 🟢 这里从 24 改为 72 小时
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析 token，返回 claims
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, jwt.ErrInvalidKey
	}
	return claims, nil
}
