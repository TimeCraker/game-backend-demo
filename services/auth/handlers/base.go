package handlers

import "gorm.io/gorm"

// DB 是全局数据库大管家，给 register, login, websocket 共同使用
var DB *gorm.DB
