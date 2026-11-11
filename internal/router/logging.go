package router

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/pkg/observability"
	"YuLing_senior_high_school/pkg/resp"
)

func validRequestID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		isLetter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		isNumber := r >= '0' && r <= '9'
		if !isLetter && !isNumber && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func requestLogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !validRequestID(id) {
			id = rand.Text()
		}
		c.Header("X-Request-ID", id)
		c.Set("request_id", id)
		c.Request = c.Request.WithContext(observability.WithLogger(c.Request.Context(), slog.Default().With("request_id", id)))
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		fields := []any{"event", "http_request", "method", c.Request.Method, "route", route,
			"status", status, "duration_ms", time.Since(start).Milliseconds(), "response_bytes", c.Writer.Size()}
		if code, ok := c.Get("business_code"); ok {
			fields = append(fields, "business_code", code)
		}
		if id, ok := c.Get("user_id"); ok {
			fields = append(fields, "user_id", id)
		}
		if id := c.Param("id"); id != "" {
			fields = append(fields, "target_id", safeTargetID(id))
		}
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		case route == "/health" || route == "/ready" || strings.HasPrefix(route, "/assets/"):
			level = slog.LevelDebug
		}
		observability.Logger(c.Request.Context()).Log(c.Request.Context(), level, "HTTP 请求", fields...)
	}
}

func safeTargetID(value string) string {
	if len(value) > 20 {
		return "invalid"
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return "invalid"
		}
	}
	return value
}

func recoveryLogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if value := recover(); value != nil {
				// Panic values, request dumps and headers can contain credentials. Keep only type and call stack.
				observability.Logger(c.Request.Context()).Error("请求发生异常",
					"event", "panic", "route", c.FullPath(), "panic_type", fmt.Sprintf("%T", value), "stack", string(debug.Stack()))
				if !c.Writer.Written() {
					resp.Internal(c, "服务暂时不可用，请稍后重试")
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
