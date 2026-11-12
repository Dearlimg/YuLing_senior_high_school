package controller

import (
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
	"github.com/gin-gonic/gin"
)

func (h *AuthController) AdminUsers(c *gin.Context) {
	var req logic.ContentRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	rows, err := h.logic.AdminUsers(c.Request.Context(), req)
	respond(c, rows, err)
}

func (h *AuthController) SetAdmin(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.AdminRoleRequest
	if c.ShouldBindJSON(&req) != nil {
		resp.BadRequest(c, "请填写管理员状态和操作原因")
		return
	}
	err := h.logic.SetAdmin(c.Request.Context(), id, req)
	if err == nil {
		auditOperation(c, "user.role", id, 0)
	}
	respond(c, gin.H{"updated": true}, err)
}

func (h *CommunityController) AdminContent(c *gin.Context) {
	var req logic.ContentRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	rows, err := h.logic.AdminContent(c.Request.Context(), c.Param("kind"), req)
	respond(c, rows, err)
}

func (h *CommunityController) AdminDelete(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.ModerationRequest
	if c.ShouldBindJSON(&req) != nil {
		resp.BadRequest(c, "请填写删除原因")
		return
	}
	req.Action = "deleted"
	err := h.logic.Moderate(c.Request.Context(), c.Param("kind"), id, req)
	if err == nil {
		auditOperation(c, "moderation."+c.Param("kind")+".deleted", id, 0)
	}
	respond(c, gin.H{"deleted": true}, err)
}
