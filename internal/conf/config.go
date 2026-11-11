// Package conf 负责应用配置的加载与校验。
//
// 设计约束：
//  1. 配置**只**来源于环境变量，不读取本地配置文件，避免配置散落在镜像里。
//  2. K8s 环境下由 ConfigMap（非敏感）与 Secret（敏感）通过 envFrom 注入；
//     本地开发用 .env.example 导出同名变量即可，代码无需感知环境差异。
//  3. 启动时集中校验，缺失必填项直接 fail-fast，不做默认可用的猜测。
package conf

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是应用全部运行期配置的聚合根。
type Config struct {
	App   App
	MySQL MySQL
	Redis Redis
}

// App 应用层配置。
type App struct {
	// Env 运行环境：dev / test / prod，影响日志与 Gin 模式。
	Env string
	// Port HTTP 监听端口。
	Port string
	// LogLevel 日志级别：debug / info / warn / error。
	LogLevel string
	// ShutdownTimeout 优雅退出时等待存量请求结束的最长时间。
	ShutdownTimeout time.Duration
	DemoMode        bool
	AdminToken      string
	TrustedProxies  []string
}

// MySQL 数据库连接配置。
type MySQL struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	// Params DSN 附加参数，例如 charset=utf8mb4&parseTime=True&loc=Local
	Params string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN 拼装 go-sql-driver 使用的连接串。
func (m MySQL) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?%s",
		m.Username, m.Password, m.Host, m.Port, m.Database, m.Params,
	)
}

// Redis 缓存连接配置。
type Redis struct {
	Addr     string
	Password string
	DB       int
	// KeyPrefix 所有键的统一前缀。共享 Redis 实例时用它做逻辑隔离，
	// 避免与其他业务的键空间互相污染。
	KeyPrefix string
	PoolSize  int
}

// Load 从环境变量装配配置并校验。
func Load() (*Config, error) {
	cfg := &Config{
		App: App{
			Env:             getEnv("APP_ENV", "dev"),
			Port:            getEnv("SERVER_PORT", "8080"),
			LogLevel:        getEnv("LOG_LEVEL", "info"),
			ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
			DemoMode:        getEnv("DEMO_MODE", "false") == "true",
			AdminToken:      os.Getenv("ADMIN_TOKEN"),
			TrustedProxies:  splitList(os.Getenv("TRUSTED_PROXIES")),
		},
		MySQL: MySQL{
			Host:            getEnv("MYSQL_HOST", "127.0.0.1"),
			Port:            getEnv("MYSQL_PORT", "3306"),
			Database:        getEnv("MYSQL_DATABASE", "yulin"),
			Username:        getEnv("MYSQL_USER", "yulin"),
			Password:        os.Getenv("MYSQL_PASSWORD"),
			Params:          getEnv("MYSQL_PARAMS", "charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"),
			MaxOpenConns:    getInt("MYSQL_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getInt("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getDuration("MYSQL_CONN_MAX_LIFETIME", time.Hour),
		},
		Redis: Redis{
			Addr:      getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password:  os.Getenv("REDIS_PASSWORD"),
			DB:        getInt("REDIS_DB", 1),
			KeyPrefix: getEnv("REDIS_KEY_PREFIX", "yulin:"),
			PoolSize:  getInt("REDIS_POOL_SIZE", 10),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate 校验必填项。密码允许为空（本地无密码 Redis），
// 但数据库名、用户名这类结构性字段不允许缺失。
func (c *Config) validate() error {
	if c.IsProd() && c.App.DemoMode {
		return fmt.Errorf("生产环境禁止开启 DEMO_MODE")
	}
	if c.App.AdminToken != "" && len(c.App.AdminToken) < 32 {
		return fmt.Errorf("ADMIN_TOKEN 至少需要 32 个字符")
	}
	var missing []string
	if c.MySQL.Host == "" {
		missing = append(missing, "MYSQL_HOST")
	}
	if c.MySQL.Database == "" {
		missing = append(missing, "MYSQL_DATABASE")
	}
	if c.MySQL.Username == "" {
		missing = append(missing, "MYSQL_USER")
	}
	if c.Redis.Addr == "" {
		missing = append(missing, "REDIS_ADDR")
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少必需的环境变量: %s", strings.Join(missing, ", "))
	}

	// 生产环境强制要求数据库密码，防止误用空密码连接到共享实例。
	if c.IsProd() && c.MySQL.Password == "" {
		return fmt.Errorf("生产环境必须提供 MYSQL_PASSWORD（应由 Secret 注入）")
	}
	return nil
}

func splitList(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// IsProd 判断是否生产环境。
func (c *Config) IsProd() bool { return c.App.Env == "prod" }

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
