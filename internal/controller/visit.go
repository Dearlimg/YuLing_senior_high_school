package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/observability"
	"YuLing_senior_high_school/pkg/resp"
)

// VisitorController 访客统计：公开上报 + 管理员看板。
type VisitorController struct {
	logic *logic.VisitLogic
	meter dao.TrafficMeter
}

func NewVisitorController(logic *logic.VisitLogic, meter dao.TrafficMeter) *VisitorController {
	return &VisitorController{logic: logic, meter: meter}
}

// Pageview POST /api/v1/track/pageview
func (h *VisitorController) Pageview(c *gin.Context) {
	var req logic.PageviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "访问统计参数格式不正确")
		return
	}
	respond(c, nil, h.logic.RecordPageview(c.Request.Context(), req, c.ClientIP(), c.Request.UserAgent()))
}

// Overview GET /api/v1/admin/visitors
func (h *VisitorController) Overview(c *gin.Context) {
	data, err := h.logic.Overview(c.Request.Context())
	if err != nil {
		respond(c, nil, err)
		return
	}
	// 底账与真实访客分开展示：底账含爬虫，仅作对照。
	traffic := gin.H{"available": false}
	if h.meter != nil {
		requests, ips, meterErr := h.meter.Today(c.Request.Context(), time.Now().UTC())
		if meterErr != nil {
			observability.Logger(c.Request.Context()).Warn("流量底账读取失败", "event", "visit_meter_error")
		} else {
			traffic = gin.H{"available": true, "requests": requests, "ips": ips}
		}
	}
	respond(c, gin.H{"visits": data, "traffic": traffic}, nil)
}
