// Package controller HTTP 层。
//
// 边界约定：只做三件事——解析入参、调用 logic、写出响应。
// 任何"该不该允许""怎么算"的判断都不应出现在这里，
// 否则同一套规则会在 HTTP 和 RPC 两个入口各写一遍并逐渐不一致。
package controller

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

// AlumniController 校友相关接口。
type AlumniController struct {
	logic *logic.AlumniLogic
}

// NewAlumniController 构造。
func NewAlumniController(l *logic.AlumniLogic) *AlumniController {
	return &AlumniController{logic: l}
}

// List GET /api/v1/alumni
func (h *AlumniController) List(c *gin.Context) {
	var req logic.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数格式错误: "+err.Error())
		return
	}

	result, err := h.logic.List(c.Request.Context(), req)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, result)
}

// GetByID GET /api/v1/alumni/:id
func (h *AlumniController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.BadRequest(c, "id 必须为正整数")
		return
	}

	vo, err := h.logic.GetByID(c.Request.Context(), id)
	if errors.Is(err, logic.ErrNotFound) {
		resp.NotFound(c, "校友不存在")
		return
	}
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, vo)
}

// Create POST /api/v1/alumni
func (h *AlumniController) Create(c *gin.Context) {
	var req logic.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请求体格式错误: "+err.Error())
		return
	}

	id, err := h.logic.Create(c.Request.Context(), req)
	if errors.Is(err, logic.ErrInvalidParam) {
		resp.BadRequest(c, err.Error())
		return
	}
	if err != nil {
		resp.Internal(c, "创建失败")
		return
	}
	resp.OK(c, gin.H{"id": id})
}
