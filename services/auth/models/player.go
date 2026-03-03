package models

import "gorm.io/gorm"

type PlayerPosition struct {
	gorm.Model
	UserID uint    `gorm:"uniqueIndex"` // 每个玩家唯一的坐标记录
	X      float64 `json:"x"`           // 坐标用 float64
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
}
