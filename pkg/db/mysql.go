// Package db 负责基础设施客户端（MySQL / Redis）的初始化与生命周期管理。
// 只在这里出现驱动细节，上层 dao 拿到的始终是抽象后的句柄。
package db

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"YuLing_senior_high_school/internal/conf"
)

// NewMySQL 建立 GORM 连接并配置连接池。
// 注意：gorm.Open 默认是懒连接，这里显式 Ping 一次，
// 让配置错误在启动期暴露，而不是拖到第一个请求才 500。
func NewMySQL(cfg conf.MySQL, isProd bool) (*gorm.DB, error) {
	logLevel := logger.Info
	if isProd {
		logLevel = logger.Warn
	}

	gdb, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		// 表名不做复数化处理，表结构由 SQL 迁移文件显式定义，避免 GORM 擅自猜测。
		NamingStrategy: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 *sql.DB 失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("MySQL 探活失败(%s:%s/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}

	return gdb, nil
}

// CloseMySQL 关闭连接池，供优雅退出调用。
func CloseMySQL(gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
