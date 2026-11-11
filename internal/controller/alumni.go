// Package controller binds HTTP parameters and translates business results.
package controller

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

type AlumniController struct{ logic *logic.AlumniLogic }

func NewAlumniController(l *logic.AlumniLogic) *AlumniController { return &AlumniController{logic: l} }

func (h *AlumniController) List(c *gin.Context) {
	var req logic.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数格式不正确")
		return
	}
	result, err := h.logic.List(c.Request.Context(), req)
	respond(c, result, err)
}
func (h *AlumniController) GetByID(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	result, err := h.logic.GetByID(c.Request.Context(), id)
	respond(c, result, err)
}
func (h *AlumniController) Create(c *gin.Context) {
	var req logic.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请求体格式不正确，请检查填写内容")
		return
	}
	result, err := h.logic.Create(c.Request.Context(), req, c.GetHeader("Idempotency-Key"))
	respond(c, result, err)
}
func (h *AlumniController) Update(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请求体格式不正确")
		return
	}
	result, err := h.logic.Update(c.Request.Context(), id, req)
	respond(c, result, err)
}
func (h *AlumniController) Unlock(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req struct {
		EditCode string `json:"edit_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请提供编辑码")
		return
	}
	result, err := h.logic.Unlock(c.Request.Context(), id, req.EditCode)
	respond(c, result, err)
}
func (h *AlumniController) Meta(c *gin.Context) {
	result, err := h.logic.Meta(c.Request.Context())
	respond(c, result, err)
}
func (h *AlumniController) Classes(c *gin.Context) {
	var req logic.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数格式不正确")
		return
	}
	result, err := h.logic.Classes(c.Request.Context(), req)
	respond(c, result, err)
}
func (h *AlumniController) Stats(c *gin.Context) {
	result, err := h.logic.Stats(c.Request.Context())
	respond(c, result, err)
}
func (h *AlumniController) Universities(c *gin.Context) {
	rows, total, err := h.logic.Universities(c.Request.Context())
	respond(c, gin.H{"items": rows, "total": total}, err)
}

func requestID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.BadRequest(c, "id 必须为正整数")
		return 0, false
	}
	return id, true
}

func respond(c *gin.Context, data any, err error) {
	switch {
	case err == nil:
		resp.OK(c, data)
	case errors.Is(err, logic.ErrImageTooLarge):
		resp.Fail(c, http.StatusRequestEntityTooLarge, 41301, err.Error())
	case errors.Is(err, logic.ErrInvalidParam):
		resp.BadRequest(c, err.Error())
	case errors.Is(err, logic.ErrNotFound):
		resp.NotFound(c, "内容不存在或已撤下")
	case errors.Is(err, logic.ErrUnauthorized):
		resp.Fail(c, http.StatusUnauthorized, 40102, "账号或密码不正确，或登录已过期，请重新登录")
	case errors.Is(err, logic.ErrForbidden):
		resp.Fail(c, http.StatusForbidden, 40301, err.Error())
	case errors.Is(err, logic.ErrDuplicate):
		resp.Fail(c, http.StatusConflict, 40901, err.Error())
	default:
		// Do not log payloads or SQL errors that may contain contact or secret values.
		slog.Error("业务操作失败", "route", c.FullPath())
		resp.Internal(c, "服务暂时不可用，请稍后重试")
	}
}
