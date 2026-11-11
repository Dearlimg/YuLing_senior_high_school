// Package router 集中注册路由。
// 单独成包是为了让 main.go 只负责装配与生命周期，不被路由细节淹没。
package router

import (
	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/conf"
	"YuLing_senior_high_school/internal/controller"
	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/internal/web"
	"YuLing_senior_high_school/pkg/resp"
)

// Deps 路由依赖集合。用结构体传参而非一堆位置参数，
// 后续增删依赖时调用方不必改签名。
type Deps struct {
	Config    *conf.Config
	Health    *controller.HealthController
	Alumni    *controller.AlumniController
	Community *controller.CommunityController
	Limiter   dao.Limiter
	Auth      *controller.AuthController
	AuthLogic *logic.AuthLogic
}

// New 构建 gin 引擎并注册全部路由。
func New(d Deps) *gin.Engine {
	if d.Config.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 生产环境不挂 pprof / 详细日志中间件；Recovery 必开，避免 panic 打挂进程。
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	if err := r.SetTrustedProxies(d.Config.App.TrustedProxies); err != nil {
		panic(err)
	}
	r.MaxMultipartMemory = 12 << 20

	// 探针路由不放在 /api 下，避免被网关的鉴权中间件拦截。
	r.GET("/health", d.Health.Health)
	r.GET("/ready", d.Health.Ready)

	v1 := r.Group("/api/v1")
	v1.Use(writeGuard(d.Limiter))
	{
		v1.GET("/meta", func(c *gin.Context) { resp.OK(c, gin.H{"demo": d.Config.App.DemoMode}) })
		v1.GET("/stats", d.Alumni.Stats)
		v1.GET("/directory-meta", d.Alumni.Meta)
		v1.GET("/classes", d.Alumni.Classes)
		v1.GET("/universities", d.Alumni.Universities)
		alumni := v1.Group("/alumni")
		{
			alumni.GET("", d.Alumni.List)
			alumni.GET("/:id", d.Alumni.GetByID)
			alumni.POST("", d.Auth.Register)
			alumni.PUT("/:id", d.Alumni.Update)
			alumni.POST("/:id/unlock", d.Alumni.Unlock)
		}
		v1.GET("/photos", d.Community.Photos)
		v1.POST("/auth/register", d.Auth.Register)
		v1.POST("/auth/login", d.Auth.Login)
		signed := v1.Group("", userGuard(d.AuthLogic))
		signed.GET("/me", d.Auth.Me)
		signed.PUT("/me/alumni", d.Auth.UpdateCard)
		signed.POST("/auth/logout", d.Auth.Logout)
		signed.POST("/photos", d.Community.Upload)
		signed.DELETE("/photos/:id", d.Community.DeletePhoto)
		v1.GET("/photos/:id/image", d.Community.PhotoImage)
		v1.GET("/posts", d.Community.Posts)
		signed.POST("/posts", d.Community.CreatePost)
		v1.GET("/posts/:id/image", d.Community.PostImage)
		v1.GET("/posts/:id/replies", d.Community.Replies)
		signed.POST("/posts/:id/replies", d.Community.Reply)
		admin := v1.Group("/admin", adminGuard(d.Config.App.AdminToken))
		admin.PUT("/:kind/:id", d.Community.Moderate)
		admin.GET("/:kind/:id", d.Community.Inspect)
		admin.GET("/audit", d.Community.AuditLogs)
	}
	web.Register(r)

	return r
}
