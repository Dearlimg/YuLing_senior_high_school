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

	// Access and panic logs share the same request ID without dumping headers or request bodies.
	r := gin.New()
	r.Use(requestLogging(), recoveryLogging())
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
		v1.GET("/class-hub", d.Alumni.ClassHub)
		v1.GET("/annual-report", d.Alumni.AnnualReport)
		v1.GET("/cohorts/:year/universities", d.Alumni.CohortUniversities)
		v1.POST("/recovery/:id/status", d.Alumni.RecoveryReceipt)
		v1.POST("/recovery/:id/complete", d.Alumni.RecoveryReceipt)
		v1.GET("/universities", d.Alumni.Universities)
		v1.POST("/growth/events", d.Alumni.GrowthEvent)
		alumni := v1.Group("/alumni")
		{
			alumni.GET("", d.Alumni.List)
			alumni.GET("/:id", d.Alumni.GetByID)
			alumni.POST("", d.Auth.Register)
			alumni.POST("/claim", d.Alumni.Claim)
			alumni.PUT("/:id", d.Alumni.Update)
			alumni.POST("/:id/unlock", d.Alumni.Unlock)
			alumni.POST("/:id/recovery", d.Alumni.RequestRecovery)
		}
		v1.GET("/photos", d.Community.Photos)
		v1.POST("/auth/register", d.Auth.Register)
		v1.POST("/auth/login", d.Auth.Login)
		signed := v1.Group("", userGuard(d.AuthLogic))
		signed.GET("/me", d.Auth.Me)
		signed.GET("/me/subscriptions", d.Alumni.Subscriptions)
		signed.PUT("/me/subscriptions", d.Alumni.Subscribe)
		signed.GET("/me/updates", d.Alumni.Updates)
		signed.PUT("/me/updates/read", d.Alumni.ReadUpdates)
		signed.GET("/me/organizer", d.Alumni.Organizers)
		signed.POST("/me/organizer", d.Alumni.ApplyOrganizer)
		signed.DELETE("/me/organizer/:id", d.Alumni.ReviewOrganizer)
		signed.PUT("/me/alumni", d.Auth.UpdateCard)
		signed.POST("/auth/logout", d.Auth.Logout)
		signed.POST("/photos", d.Community.Upload)
		signed.DELETE("/photos/:id", d.Community.DeletePhoto)
		v1.GET("/photos/:id", d.Community.Photo)
		signed.GET("/photos/:id/likes", d.Community.PhotoLikes)
		signed.PUT("/photos/:id/likes", d.Community.PhotoLikes)
		v1.GET("/photos/:id/replies", d.Community.PhotoReplies)
		signed.POST("/photos/:id/replies", d.Community.PhotoReply)
		v1.GET("/photos/:id/image", d.Community.PhotoImage)
		v1.GET("/posts", d.Community.Posts)
		v1.GET("/posts/ranking", d.Community.Ranking)
		signed.POST("/posts", d.Community.CreatePost)
		v1.GET("/posts/:id", d.Community.Post)
		signed.GET("/posts/:id/likes", d.Community.PostLikes)
		signed.PUT("/posts/:id/likes", d.Community.PostLikes)
		v1.GET("/posts/:id/image", d.Community.PostImage)
		v1.GET("/posts/:id/replies", d.Community.Replies)
		signed.POST("/posts/:id/replies", d.Community.Reply)
		admin := v1.Group("/admin", adminGuard(d.Config.App.AdminToken, d.AuthLogic))
		admin.GET("/users", d.Auth.AdminUsers)
		admin.GET("/growth", d.Alumni.GrowthMetrics)
		admin.GET("/organizers", d.Alumni.Organizers)
		admin.PUT("/organizers/:id", d.Alumni.ReviewOrganizer)
		admin.GET("/recoveries", d.Alumni.Recoveries)
		admin.PUT("/recoveries/:id", d.Alumni.ReviewRecovery)
		admin.PUT("/users/:id/role", d.Auth.SetAdmin)
		admin.GET("/content/:kind", d.Community.AdminContent)
		admin.DELETE("/:kind/:id", d.Community.AdminDelete)
		admin.PUT("/:kind/:id", d.Community.Moderate)
		admin.GET("/:kind/:id", d.Community.Inspect)
		admin.GET("/audit", d.Community.AuditLogs)
	}
	web.Register(r)

	return r
}
