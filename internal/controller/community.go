package controller

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"YuLing_senior_high_school/internal/logic"
	"YuLing_senior_high_school/pkg/resp"
)

type CommunityController struct{ logic *logic.CommunityLogic }

func NewCommunityController(l *logic.CommunityLogic) *CommunityController {
	return &CommunityController{logic: l}
}

func (h *CommunityController) Photos(c *gin.Context) {
	var req logic.ContentRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	result, err := h.logic.Photos(c.Request.Context(), req)
	respond(c, result, err)
}
func (h *CommunityController) Upload(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(12 << 20); err != nil {
		resp.BadRequest(c, "请使用图片表单上传，原图不能超过10MB")
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	image, err := readImage(c, true)
	if err != nil {
		respond(c, nil, err)
		return
	}
	result, err := h.logic.Upload(c.Request.Context(), logic.PhotoRequest{
		Caption: c.PostForm("caption"), Uploader: c.PostForm("uploader"),
		Category: c.PostForm("category"), Image: image, Key: c.GetHeader("Idempotency-Key"),
	})
	respond(c, result, err)
}

func (h *CommunityController) DeletePhoto(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req struct {
		DeleteCode string `json:"delete_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "请提供删除码")
		return
	}
	err := h.logic.DeletePhoto(c.Request.Context(), id, req.DeleteCode)
	respond(c, gin.H{"deleted": true}, err)
}
func (h *CommunityController) PhotoImage(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	data, mime, err := h.logic.PhotoImage(c.Request.Context(), id)
	serveImage(c, data, mime, err)
}
func (h *CommunityController) Posts(c *gin.Context) {
	var req logic.ContentRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	result, err := h.logic.Posts(c.Request.Context(), req)
	respond(c, result, err)
}
func (h *CommunityController) CreatePost(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(12 << 20); err != nil {
		resp.BadRequest(c, "发帖请求格式不正确")
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	year := 0
	if value := c.PostForm("graduation_year"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			resp.BadRequest(c, "届别必须为数字年份")
			return
		}
		year = parsed
	}
	image, err := readImage(c, false)
	if err != nil {
		respond(c, nil, err)
		return
	}
	id, err := h.logic.CreatePost(c.Request.Context(), logic.PostRequest{
		Nickname: c.PostForm("nickname"), GraduationYear: year,
		Content: c.PostForm("content"), Image: image, Key: c.GetHeader("Idempotency-Key"),
	})
	respond(c, gin.H{"id": id}, err)
}
func (h *CommunityController) PostImage(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	data, mime, err := h.logic.PostImage(c.Request.Context(), id)
	serveImage(c, data, mime, err)
}
func (h *CommunityController) Replies(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.ContentRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	result, err := h.logic.Replies(c.Request.Context(), id, req)
	respond(c, result, err)
}
func (h *CommunityController) Reply(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.ReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "回复格式不正确")
		return
	}
	count, err := h.logic.Reply(c.Request.Context(), id, req, c.GetHeader("Idempotency-Key"))
	respond(c, gin.H{"reply_count": count}, err)
}
func (h *CommunityController) Moderate(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var req logic.ModerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "管理请求格式不正确")
		return
	}
	err := h.logic.Moderate(c.Request.Context(), c.Param("kind"), id, req)
	respond(c, gin.H{"updated": true}, err)
}
func (h *CommunityController) AuditLogs(c *gin.Context) {
	var req logic.ContentRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.BadRequest(c, "查询参数不正确")
		return
	}
	rows, err := h.logic.AuditLogs(c.Request.Context(), req)
	respond(c, gin.H{"items": rows}, err)
}

func readImage(c *gin.Context, required bool) ([]byte, error) {
	header, err := c.FormFile("image")
	if err != nil {
		if required {
			return nil, logic.ErrInvalidParam
		}
		return nil, nil
	}
	if header.Size > 10<<20 {
		return nil, logic.ErrInvalidParam
	}
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 10<<20 {
		return nil, logic.ErrInvalidParam
	}
	return data, nil
}

func (h *CommunityController) Inspect(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	item, err := h.logic.Inspect(c.Request.Context(), c.Param("kind"), id)
	respond(c, item, err)
}
func serveImage(c *gin.Context, data []byte, mime string, err error) {
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(200, mime, data)
}
