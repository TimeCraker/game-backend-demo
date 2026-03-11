package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex;size:50;not null"`
	Password string `gorm:"size:255;not null"` // 存储加密后的密码
	// 新增邮箱字段，设定为唯一索引，确保一个邮箱只能注册一个账号
	Email string `gorm:"uniqueIndex;size:100;not null"`
}
