package controller

import (
	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/pkg/resp"
)

func (h *CommunityController) Post(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	row, err := h.logic.Post(c.Request.Context(), id)
	respond(c, row, err)
}

func (h *CommunityController) PostLikes(c *gin.Context) { h.contentLikes(c, "post") }

func (h *CommunityController) contentLikes(c *gin.Context, kind string) {
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
	likes := h.logic.PostLikes
	if kind == "photo" {
		likes = h.logic.PhotoLikes
	}
	state, err := likes(c.Request.Context(), id, liked)
	respond(c, state, err)
}

func (h *CommunityController) Photo(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	row, err := h.logic.Photo(c.Request.Context(), id)
	respond(c, row, err)
}
func (h *CommunityController) PhotoLikes(c *gin.Context)   { h.contentLikes(c, "photo") }
func (h *CommunityController) PhotoReplies(c *gin.Context) { h.contentReplies(c, "photo") }
func (h *CommunityController) PhotoReply(c *gin.Context)   { h.contentReply(c, "photo") }
