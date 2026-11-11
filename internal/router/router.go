// Package router 集中注册路由。
// 单独成包是为了让 main.go 只负责装配与生命周期，不被路由细节淹没。
package router

import (
	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/controller"
	"YuLing_senior_high_school/internal/conf"
)

// Deps 路由依赖集合。用结构体传参而非一堆位置参数，
// 后续增删依赖时调用方不必改签名。
type Deps struct {
	Config     *conf.Config
	Health     *controller.HealthController
	Alumni     *controller.AlumniController
}

// New 构建 gin 引擎并注册全部路由。
func New(d Deps) *gin.Engine {
	if d.Config.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 生产环境不挂 pprof / 详细日志中间件；Recovery 必开，避免 panic 打挂进程。
	r := gin.New()
	r.Use(gin.Recovery())

	// 探针路由不放在 /api 下，避免被网关的鉴权中间件拦截。
	r.GET("/health", d.Health.Health)
	r.GET("/ready", d.Health.Ready)

	v1 := r.Group("/api/v1")
	{
		alumni := v1.Group("/alumni")
		{
			alumni.GET("", d.Alumni.List)
			alumni.GET("/:id", d.Alumni.GetByID)
			alumni.POST("", d.Alumni.Create)
		}
	}

	return r
}
