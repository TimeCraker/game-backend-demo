package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex;size:50;not null"`
	Password string `gorm:"size:255;not null"` // 存储加密后的密码
}
