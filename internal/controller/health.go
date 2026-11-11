package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"YuLing_senior_high_school/pkg/observability"
)

// HealthController 健康检查。
//
// 两个探针的语义必须分开，混用会导致 K8s 误杀：
//   - /health（liveness）：只回答"进程还活着吗"。绝不能查数据库，
//     否则 MySQL 抖动会让 K8s 重启一堆本来健康的 Pod，故障放大。
//   - /ready（readiness）：回答"能不能接流量"，此时才检查依赖。
type HealthController struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewHealthController 构造。
func NewHealthController(db *gorm.DB, rdb *redis.Client) *HealthController {
	return &HealthController{db: db, rdb: rdb}
}

// Health GET /health
func (h *HealthController) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

// Ready GET /ready
func (h *HealthController) Ready(c *gin.Context) {
	if h.db == nil && h.rdb == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "mode": "demo", "persistence": "ephemeral"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		readinessError(c, "mysql", "no-handle", err)
		return
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		readinessError(c, "mysql", "down", err)
		return
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		readinessError(c, "redis", "down", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "mysql": "up", "redis": "up"})
}

func readinessError(c *gin.Context, dependency, state string, err error) {
	observability.Logger(c.Request.Context()).Warn("就绪检查失败", append([]any{"event", "readiness_error", "dependency", dependency}, observability.ErrorFields(err)...)...)
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", dependency: state})
}
