// Command server 是单体服务的唯一入口。
//
// 装配顺序刻意写成"配置 → 基础设施 → DAO → Logic → Controller → Router"，
// 依赖方向单向向下，任何一层都不回头依赖上层。
// 一旦出现反向依赖，说明该逻辑放错了层。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// 把 IANA 时区数据库编进二进制（约 450KB）。
	// 这样容器里即使没装 tzdata，TZ=Asia/Shanghai 也能正确生效；
	// 否则 Go 读不到 /usr/share/zoneinfo 会静默退回 UTC，日志时间整体偏移 8 小时，
	// 而且这种错误在日志里看不出来，只能靠对上真实时间才发现。
	_ "time/tzdata"

	"YuLing_senior_high_school/internal/conf"
	"YuLing_senior_high_school/internal/controller"
	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/internal/router"
	"YuLing_senior_high_school/pkg/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("服务启动失败", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. 配置：缺失必填项直接退出，不带着半截配置启动。
	cfg, err := conf.Load()
	if err != nil {
		return err
	}
	initLogger(cfg)

	slog.Info("配置加载完成",
		"env", cfg.App.Env,
		"port", cfg.App.Port,
		"mysql", cfg.MySQL.Host+":"+cfg.MySQL.Port+"/"+cfg.MySQL.Database,
		"redis", cfg.Redis.Addr,
		"redis_db", cfg.Redis.DB,
	)

	// 2. 基础设施。任一依赖不可用都不启动，
	//    宁可 CrashLoop 让 K8s 重试，也不要带着坏连接对外提供服务。
	gdb, err := db.NewMySQL(cfg.MySQL, cfg.IsProd())
	if err != nil {
		return err
	}
	defer func() { _ = db.CloseMySQL(gdb) }()

	rdb, err := db.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = db.CloseRedis(rdb) }()

	// 3. 分层装配。
	alumniDAO := dao.NewAlumniDAO(gdb)
	alumniLogic := logic.NewAlumniLogic(alumniDAO)

	engine := router.New(router.Deps{
		Config: cfg,
		Health: controller.NewHealthController(gdb, rdb),
		Alumni: controller.NewAlumniController(alumniLogic),
	})

	srv := &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 4. 收到 SIGTERM 后停止接受新连接，并给存量请求留出退出时间。
	//    K8s 滚动更新依赖这个行为，否则每次发布都会打断在途请求。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP 服务已启动", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("服务已退出")
	return nil
}

// initLogger 按环境选择日志形态：生产用 JSON 便于采集，开发用文本便于阅读。
func initLogger(cfg *conf.Config) {
	level := slog.LevelInfo
	switch cfg.App.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.IsProd() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}
