package controller

import (
	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

func (h *AlumniController) Claim(c *gin.Context) {
	var req logic.ClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请求体格式不正确")
		return
	}
	result, err := h.logic.Claim(c.Request.Context(), req, c.GetHeader("Idempotency-Key"))
	respond(c, result, err)
}

func (h *AlumniController) GrowthEvent(c *gin.Context) {
	var req logic.GrowthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "统计参数格式不正确")
		return
	}
	respond(c, nil, h.logic.RecordGrowth(c.Request.Context(), req))
}

func (h *AlumniController) GrowthMetrics(c *gin.Context) {
	var req logic.GrowthMetricsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "统计查询格式不正确")
		return
	}
	result, err := h.logic.GrowthMetrics(c.Request.Context(), req)
	respond(c, result, err)
}
