package router

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
	"errors"
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
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 12<<20)
		limit := 20
		period := 10 * time.Minute
		if strings.HasSuffix(c.FullPath(), "/unlock") || strings.HasPrefix(c.FullPath(), "/api/v1/auth/") {
			limit = 10
			period = time.Minute
		}
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(c.ClientIP()+":"+c.FullPath())))
		allowed, err := limiter.Allow(c.Request.Context(), key, limit, period)
		if err != nil {
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

func adminGuard(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			resp.Fail(c, 503, 50302, "管理功能尚未配置")
			c.Abort()
			return
		}
		provided := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		a, b := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
			resp.Fail(c, 401, 40101, "需要管理员凭据")
			c.Abort()
			return
		}
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
