package models

import (
	"time"
)

// Message 对应数据库中的 messages 表
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Sender    string    `gorm:"size:255;not null" json:"sender"`   // 发送者用户名
	Content   string    `gorm:"type:text;not null" json:"content"` // 消息内容
	CreatedAt time.Time `json:"created_at"`                        // 创建时间
}
