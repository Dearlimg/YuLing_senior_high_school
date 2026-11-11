// Command migrate applies additive GORM schema migrations to the configured MySQL database.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"YuLing_senior_high_school/internal/conf"
	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/pkg/db"
)

func main() {
	if err := migrate(); err != nil {
		slog.Error("数据库迁移失败，请检查配置和表结构")
		os.Exit(1)
	}
	slog.Info("GORM 数据库迁移完成")
}

func migrate() error {
	cfg, err := conf.Load()
	if err != nil {
		return err
	}
	connection, err := db.NewMySQL(cfg.MySQL, cfg.IsProd())
	if err != nil {
		return err
	}
	defer func() { _ = db.CloseMySQL(connection) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return dao.NewSQLStore(connection).Migrate(ctx)
}
