package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
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
	ctx := c.Request.Context()

	sqlDB, err := h.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "mysql": "no-handle", "detail": err.Error()})
		return
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "mysql": "down", "detail": err.Error()})
		return
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "redis": "down", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "mysql": "up", "redis": "up"})
}
