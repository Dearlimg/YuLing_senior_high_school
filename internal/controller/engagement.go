package controller

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

func bindGrowthJSON(c *gin.Context, req any) bool {
	if c.ShouldBindJSON(req) != nil {
		resp.BadRequest(c, "填写内容格式不正确")
		return false
	}
	return true
}

func (h *AlumniController) ClassHub(c *gin.Context) {
	var req logic.ListRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "班级参数不正确")
		return
	}
	result, err := h.logic.ClassHub(c.Request.Context(), req)
	respond(c, result, err)
}

func (h *AlumniController) ApplyOrganizer(c *gin.Context) {
	var req struct {
		Introduction string `json:"introduction"`
	}
	if !bindGrowthJSON(c, &req) {
		return
	}
	respond(c, nil, h.logic.ApplyOrganizer(c.Request.Context(), req.Introduction))
}

func (h *AlumniController) Organizers(c *gin.Context) {
	var req logic.ContentRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "分页参数不正确")
		return
	}
	mine := strings.HasPrefix(c.FullPath(), "/api/v1/me/")
	result, err := h.logic.Organizers(c.Request.Context(), req, mine)
	respond(c, result, err)
}

func (h *AlumniController) ReviewOrganizer(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	mine := strings.HasPrefix(c.FullPath(), "/api/v1/me/")
	var req logic.ReviewRequest
	if !mine && !bindGrowthJSON(c, &req) {
		return
	}
	respond(c, nil, h.logic.ReviewOrganizer(c.Request.Context(), id, req, mine))
}

func (h *AlumniController) RequestRecovery(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req struct {
		Explanation string `json:"explanation"`
	}
	if !bindGrowthJSON(c, &req) {
		return
	}
	result, err := h.logic.RequestRecovery(c.Request.Context(), id, req.Explanation)
	respond(c, result, err)
}

func (h *AlumniController) RecoveryReceipt(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req struct {
		Receipt string `json:"receipt"`
	}
	if !bindGrowthJSON(c, &req) {
		return
	}
	if strings.HasSuffix(c.FullPath(), "/complete") {
		result, err := h.logic.CompleteRecovery(c.Request.Context(), id, req.Receipt)
		respond(c, result, err)
		return
	}
	status, err := h.logic.RecoveryStatus(c.Request.Context(), id, req.Receipt)
	respond(c, gin.H{"status": status}, err)
}

func (h *AlumniController) Recoveries(c *gin.Context) {
	var req logic.ContentRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "分页参数不正确")
		return
	}
	rows, total, err := h.logic.Recoveries(c.Request.Context(), req)
	respond(c, gin.H{"items": rows, "total": total}, err)
}

func (h *AlumniController) ReviewRecovery(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.ReviewRequest
	if !bindGrowthJSON(c, &req) {
		return
	}
	respond(c, nil, h.logic.ReviewRecovery(c.Request.Context(), id, req))
}

func (h *AlumniController) CohortUniversities(c *gin.Context) {
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil {
		resp.BadRequest(c, "届别不正确")
		return
	}
	rows, total, err := h.logic.CohortUniversities(c.Request.Context(), year)
	respond(c, gin.H{"items": rows, "total": total}, err)
}
