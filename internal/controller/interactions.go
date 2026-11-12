package controller

import (
	"YuLing_senior_high_school/pkg/resp"
	"github.com/gin-gonic/gin"
)

func (h *CommunityController) Post(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	row, err := h.logic.Post(c.Request.Context(), id)
	respond(c, row, err)
}

func (h *CommunityController) PostLikes(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var liked *bool
	if c.Request.Method == "PUT" {
		var req struct {
			Liked *bool `json:"liked"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Liked == nil {
			resp.BadRequest(c, "请指定 liked 状态")
			return
		}
		liked = req.Liked
	}
	state, err := h.logic.PostLikes(c.Request.Context(), id, liked)
	respond(c, state, err)
}
