package controller

import (
	"strings"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

type AuthController struct{ logic *logic.AuthLogic }

func NewAuthController(l *logic.AuthLogic) *AuthController { return &AuthController{logic: l} }

func (h *AuthController) Register(c *gin.Context) {
	var req logic.RegisterRequest
	if c.ShouldBindJSON(&req) != nil {
		resp.BadRequest(c, "注册内容格式不正确")
		return
	}
	result, err := h.logic.Register(c.Request.Context(), req, c.GetHeader("Idempotency-Key"))
	respond(c, result, err)
}

func (h *AuthController) Login(c *gin.Context) {
	var req logic.LoginRequest
	if c.ShouldBindJSON(&req) != nil {
		resp.BadRequest(c, "请填写账号和密码")
		return
	}
	result, err := h.logic.Login(c.Request.Context(), req)
	respond(c, result, err)
}

func (h *AuthController) Logout(c *gin.Context) {
	err := h.logic.Logout(c.Request.Context(), strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	respond(c, gin.H{"logged_out": true}, err)
}

func (h *AuthController) Me(c *gin.Context) {
	result, err := h.logic.Me(c.Request.Context())
	respond(c, result, err)
}

func (h *AuthController) UpdateCard(c *gin.Context) {
	var req logic.CreateRequest
	if c.ShouldBindJSON(&req) != nil {
		resp.BadRequest(c, "校友卡内容格式不正确")
		return
	}
	result, err := h.logic.UpdateCard(c.Request.Context(), req)
	respond(c, result, err)
}
