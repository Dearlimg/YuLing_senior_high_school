package router

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/observability"
	"YuLing_senior_high_school/pkg/resp"
)

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' blob: data:; object-src 'none'; base-uri 'self'; "+
				"frame-ancestors 'none'; form-action 'self'",
		)
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}

// trafficMeter 全量流量底账：每个 HTTP 请求按天计数并记录 IP 哈希集合。
// 与真实访客指标（beacon）分离，含爬虫流量；探针路径不计入。
// 上报异步执行，Redis 抖动不影响主链路，底账允许少量丢失。
func trafficMeter(meter dao.TrafficMeter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if meter != nil && c.Request.URL.Path != "/health" && c.Request.URL.Path != "/ready" {
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(c.ClientIP()+"|"+c.Request.UserAgent())))
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = meter.Track(ctx, hash, time.Now().UTC())
			}()
		}
		c.Next()
	}
}

func writeGuard(limiter dao.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host != c.Request.Host {
				resp.Fail(c, 403, 40302, "不接受跨站提交")
				c.Abort()
				return
			}
		}
		if len(c.GetHeader("Idempotency-Key")) > 128 {
			resp.BadRequest(c, "提交标识过长")
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, logic.MaxUploadImageBytes+(128<<10))
		limit := 20
		period := 10 * time.Minute
		if c.FullPath() == "/api/v1/growth/events" || c.FullPath() == "/api/v1/track/pageview" {
			limit = 120
		}
		credentialRoute := strings.HasSuffix(c.FullPath(), "/unlock") ||
			strings.HasPrefix(c.FullPath(), "/api/v1/auth/") || strings.Contains(c.FullPath(), "/recovery")
		if credentialRoute {
			limit = 10
			period = time.Minute
		}
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(c.ClientIP()+":"+c.FullPath())))
		allowed, err := limiter.Allow(c.Request.Context(), key, limit, period)
		if err != nil {
			observability.Logger(c.Request.Context()).Error("写操作限流不可用", append([]any{"event", "rate_limit_error"}, observability.ErrorFields(err)...)...)
			resp.Fail(c, 503, 50301, "服务繁忙，请稍后重试")
			c.Abort()
			return
		}
		if !allowed {
			c.Header("Retry-After", fmt.Sprint(int(period.Seconds())))
			resp.Fail(c, 429, 42901, "操作过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

func adminGuard(token string, auth *logic.AuthLogic) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token != "" && provided != "" {
			a, b := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(provided))
			if subtle.ConstantTimeCompare(a[:], b[:]) == 1 {
				c.Set("admin", true)
				c.Request = c.Request.WithContext(logic.WithAdministrator(c.Request.Context()))
				c.Next()
				return
			}
		}
		if provided == "" {
			if token == "" {
				resp.Fail(c, 503, 50302, "请使用已授权的管理员账号登录，首次授权需配置管理 Token")
			} else {
				resp.Fail(c, 401, 40101, "请先登录管理员账号")
			}
			c.Abort()
			return
		}
		actor, err := auth.Authenticate(c.Request.Context(), provided)
		if err != nil {
			if errors.Is(err, logic.ErrUnauthorized) {
				resp.Fail(c, 401, 40102, "管理员登录已过期")
			} else {
				resp.Fail(c, 503, 50301, "权限验证暂时不可用")
			}
			c.Abort()
			return
		}
		if !actor.IsAdmin {
			resp.Fail(c, 403, 40301, "需要管理员权限")
			c.Abort()
			return
		}
		c.Set("admin", true)
		c.Set("user_id", actor.UserID)
		c.Request = c.Request.WithContext(logic.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}

func userGuard(auth *logic.AuthLogic) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			resp.Fail(c, 401, 40102, "请先登录")
			c.Abort()
			return
		}
		actor, err := auth.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			if errors.Is(err, logic.ErrUnauthorized) {
				resp.Fail(c, 401, 40102, err.Error())
			} else {
				observability.Logger(c.Request.Context()).Error("会话验证不可用", append([]any{"event", "session_error"}, observability.ErrorFields(err)...)...)
				resp.Fail(c, 503, 50301, "登录验证暂时不可用，请稍后重试")
			}
			c.Abort()
			return
		}
		c.Set("user_id", actor.UserID)
		c.Request = c.Request.WithContext(logic.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}
