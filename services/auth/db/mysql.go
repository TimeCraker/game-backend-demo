package db

import (
	"log"

	"github.com/TimeCraker/game-backend-demo/services/auth/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// SQLDB 定义为一个包级变量，方便其他地方通过 db.SQLDB 访问
var SQLDB *gorm.DB

func InitMySQL() {
	// 这里的 game_dev 是你的数据库名，rootpassword 是你的密码
	dsn := "root:rootpassword@tcp(localhost:3306)/game_dev?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ MySQL 连接失败: %v", err)
	}

	// 自动迁移：把所有的模型都在这里登记
	err = db.AutoMigrate(&models.User{}, &models.Message{})
	if err != nil {
		log.Fatalf("❌ 数据库自动迁移失败: %v", err)
	}

	SQLDB = db // 赋值给全局变量
	log.Println("✅ MySQL 初始化成功，User & Message 表已同步")
}
