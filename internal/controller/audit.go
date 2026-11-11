package controller

import (
	"YuLing_senior_high_school/pkg/observability"
	"github.com/gin-gonic/gin"
)

func auditOperation(c *gin.Context, action string, targetID, userID uint64) {
	if userID == 0 {
		userID = c.GetUint64("user_id")
	}
	fields := []any{"event", "business_operation", "action", action, "target_id", targetID, "user_id", userID}
	if c.GetBool("admin") {
		fields = append(fields, "actor_role", "admin")
	}
	observability.Logger(c.Request.Context()).Info("业务操作完成", fields...)
}
