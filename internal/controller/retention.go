package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

func (h *AlumniController) Subscribe(c *gin.Context) {
	var req logic.SubscriptionRequest
	if !bindGrowthJSON(c, &req) {
		return
	}
	respond(c, nil, h.logic.Subscribe(c.Request.Context(), req))
}

func (h *AlumniController) Subscriptions(c *gin.Context) {
	rows, err := h.logic.Subscriptions(c.Request.Context())
	respond(c, gin.H{"items": rows}, err)
}

func (h *AlumniController) Updates(c *gin.Context) {
	var req logic.ContentRequest
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "分页参数不正确")
		return
	}
	result, err := h.logic.Updates(c.Request.Context(), req)
	respond(c, result, err)
}

func (h *AlumniController) ReadUpdates(c *gin.Context) {
	var req struct {
		Through time.Time `json:"through"`
	}
	if !bindGrowthJSON(c, &req) {
		return
	}
	respond(c, nil, h.logic.ReadUpdates(c.Request.Context(), req.Through))
}

func (h *AlumniController) AnnualReport(c *gin.Context) {
	var req struct {
		Year int `form:"year"`
	}
	if c.ShouldBindQuery(&req) != nil {
		resp.BadRequest(c, "报告年份不正确")
		return
	}
	result, err := h.logic.AnnualReport(c.Request.Context(), req.Year)
	respond(c, result, err)
}
