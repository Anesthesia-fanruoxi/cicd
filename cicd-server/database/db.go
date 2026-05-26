package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	db   *sql.DB
	once sync.Once
)

// DBConfig 数据库配置
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// InitDB 初始化数据库连接
func InitDB(config DBConfig) error {
	var err error
	once.Do(func() {
		// 构建DSN (Data Source Name)
		// user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.User,
			config.Password,
			config.Host,
			config.Port,
			config.Database,
		)

		db, err = sql.Open("mysql", dsn)
		if err != nil {
			err = fmt.Errorf("打开数据库失败: %v", err)
			return
		}

		// 设置连接池参数
		db.SetMaxOpenConns(50)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(time.Hour)

		// 测试连接
		if err = db.Ping(); err != nil {
			err = fmt.Errorf("数据库连接测试失败: %v", err)
			return
		}
	})

	return err
}

// GetDB 获取数据库实例
func GetDB() *sql.DB {
	return db
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
