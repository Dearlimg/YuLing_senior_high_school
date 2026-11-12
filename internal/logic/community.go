package logic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // Required decoder registration for byte-level upload validation.
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

type CommunityLogic struct{ store dao.Store }

func NewCommunityLogic(store dao.Store) *CommunityLogic { return &CommunityLogic{store: store} }

type ContentRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Category string `form:"category"`
	Keyword  string `form:"keyword"`
	Sort     string `form:"sort"`
}

type PhotoVO struct {
	ID        uint64    `json:"id"`
	Caption   string    `json:"caption"`
	Uploader  string    `json:"uploader"`
	Category  string    `json:"category"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

type PhotoResult struct {
	Items []PhotoVO `json:"items"`
	Total int64     `json:"total"`
}
type PhotoRequest struct {
	Caption  string
	Uploader string
	Category string
	Image    []byte
	Key      string
}
type PhotoCreated struct {
	ID         uint64 `json:"id"`
	DeleteCode string `json:"delete_code"`
}

func (l *CommunityLogic) Photos(ctx context.Context, req ContentRequest) (*PhotoResult, error) {
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListPhotos(ctx, dao.ContentQuery{Page: page, PageSize: size, Category: req.Category})
	if err != nil {
		return nil, err
	}
	items := make([]PhotoVO, 0, len(rows))
	for _, r := range rows {
		items = append(items, PhotoVO{
			ID: r.ID, Caption: r.Caption, Uploader: r.Uploader, Category: r.Category,
			ImageURL: fmt.Sprintf("/api/v1/photos/%d/image", r.ID), CreatedAt: r.CreatedAt,
		})
	}
	return &PhotoResult{Items: items, Total: total}, nil
}

func (l *CommunityLogic) Upload(ctx context.Context, req PhotoRequest) (*PhotoCreated, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return nil, err
	}
	caption, uploader := strings.TrimSpace(req.Caption), actor.Card.Name
	if err := validateText("照片说明", caption, 80, true); err != nil {
		return nil, err
	}
	if err := validateText("上传人", uploader, 30, true); err != nil {
		return nil, err
	}
	if !slicesContain([]string{"校园风景", "老照片", "运动场上", "其他"}, req.Category) {
		return nil, invalid("照片分类不正确")
	}
	compressed, err := processImage(req.Image, true)
	if err != nil {
		return nil, err
	}
	code, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	row := &model.Photo{
		UserID: actor.UserID, Caption: caption, Uploader: uploader, Category: req.Category,
		Image: compressed, MIME: "image/jpeg", DeleteCodeHash: hash,
		CreateKey: actorCreateKey(actor.UserID, req.Key, req), Status: "active",
	}
	if err := l.store.CreatePhoto(ctx, row); err != nil {
		return nil, businessError(err)
	}
	return &PhotoCreated{ID: row.ID, DeleteCode: code}, nil
}

func (l *CommunityLogic) PhotoImage(ctx context.Context, id uint64) ([]byte, string, error) {
	r, err := l.store.FindPhoto(ctx, id)
	if err != nil {
		return nil, "", businessError(err)
	}
	return r.Image, r.MIME, nil
}

func (l *CommunityLogic) DeletePhoto(ctx context.Context, id uint64, code string) error {
	row, err := l.store.FindPhoto(ctx, id)
	if err != nil {
		return businessError(err)
	}
	actor, err := CurrentActor(ctx)
	if err != nil {
		return err
	}
	if (row.UserID != 0 && row.UserID != actor.UserID) || (row.UserID == 0 && !matchesSecret(code, row.DeleteCodeHash)) {
		return ErrForbidden
	}
	return businessError(l.store.DeletePhoto(ctx, id))
}

type PostVO struct {
	ID             uint64    `json:"id"`
	Nickname       string    `json:"nickname"`
	GraduationYear int       `json:"graduation_year"`
	Content        string    `json:"content"`
	ImageURL       string    `json:"image_url,omitempty"`
	AvatarColor    string    `json:"avatar_color"`
	LikeCount      int64     `json:"like_count"`
	ReplyCount     int64     `json:"reply_count"`
	CreatedAt      time.Time `json:"created_at"`
}

type PostResult struct {
	Items []PostVO `json:"items"`
	Total int64    `json:"total"`
}
type PostRequest struct {
	Nickname       string
	GraduationYear int
	Content        string
	Image          []byte
	Key            string
}

func (l *CommunityLogic) Posts(ctx context.Context, req ContentRequest) (*PostResult, error) {
	if err := validateText("搜索关键词", req.Keyword, 100, false); err != nil {
		return nil, err
	}
	if req.Sort != "" && req.Sort != "latest" && req.Sort != "hot" {
		return nil, invalid("排序方式不正确")
	}
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListPosts(ctx, dao.ContentQuery{Page: page, PageSize: size, Keyword: strings.TrimSpace(req.Keyword), Sort: req.Sort})
	if err != nil {
		return nil, err
	}
	return postResult(rows, total), nil
}
func (l *CommunityLogic) Ranking(ctx context.Context, period string) (*PostResult, error) {
	if period != "all" && period != "week" {
		return nil, invalid("榜单范围只能为 all 或 week")
	}
	rows, err := l.store.RankedPosts(ctx, period)
	if err != nil {
		return nil, err
	}
	rows = rows[:min(len(rows), 10)]
	return postResult(rows, int64(len(rows))), nil
}
func postResult(rows []model.Post, total int64) *PostResult {
	items := make([]PostVO, 0, len(rows))
	for _, r := range rows {
		v := PostVO{ID: r.ID, Nickname: r.Nickname, GraduationYear: r.GraduationYear, Content: r.Content, AvatarColor: r.AvatarColor, LikeCount: r.LikeCount, ReplyCount: r.ReplyCount, CreatedAt: r.CreatedAt}
		if r.HasImage {
			v.ImageURL = fmt.Sprintf("/api/v1/posts/%d/image", r.ID)
		}
		items = append(items, v)
	}
	return &PostResult{Items: items, Total: total}
}

func (l *CommunityLogic) CreatePost(ctx context.Context, req PostRequest) (uint64, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return 0, err
	}
	nickname, content := actor.Card.Name, strings.TrimSpace(req.Content)
	if err := validateText("昵称", nickname, 30, true); err != nil {
		return 0, err
	}
	if err := validateText("正文", content, 500, true); err != nil {
		return 0, err
	}
	if req.GraduationYear != 0 && (req.GraduationYear < 1903 || req.GraduationYear > time.Now().Year()+6) {
		return 0, invalid("届别年份不正确")
	}
	compressed, err := processImage(req.Image, false)
	if err != nil {
		return 0, err
	}
	mime := ""
	if len(compressed) > 0 {
		mime = "image/jpeg"
	}
	row := &model.Post{
		UserID: actor.UserID, Nickname: nickname, GraduationYear: actor.Card.GraduationYear, Content: content,
		Image: compressed, MIME: mime, AvatarColor: actor.Card.AvatarColor, Status: "active",
		CreateKey: actorCreateKey(actor.UserID, req.Key, req),
	}
	if err := l.store.CreatePost(ctx, row); err != nil {
		return 0, businessError(err)
	}
	return row.ID, nil
}

func (l *CommunityLogic) PostImage(ctx context.Context, id uint64) ([]byte, string, error) {
	r, err := l.store.FindPost(ctx, id)
	if err != nil {
		return nil, "", businessError(err)
	}
	if len(r.Image) == 0 {
		return nil, "", ErrNotFound
	}
	return r.Image, r.MIME, nil
}

type ReplyVO struct {
	ID        uint64    `json:"id"`
	Nickname  string    `json:"nickname"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
type ReplyResult struct {
	Items   []ReplyVO `json:"items"`
	Total   int64     `json:"total"`
	HasMore bool      `json:"has_more"`
}
type ReplyRequest struct {
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
}

func (l *CommunityLogic) Replies(ctx context.Context, id uint64, req ContentRequest) (*ReplyResult, error) {
	if _, err := l.store.FindPost(ctx, id); err != nil {
		return nil, businessError(err)
	}
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListReplies(ctx, id, dao.ContentQuery{Page: page, PageSize: size})
	if err != nil {
		return nil, businessError(err)
	}
	items := make([]ReplyVO, 0, len(rows))
	for _, r := range rows {
		items = append(items, ReplyVO{ID: r.ID, Nickname: r.Nickname, Content: r.Content, CreatedAt: r.CreatedAt})
	}
	return &ReplyResult{Items: items, Total: total, HasMore: int64(page*size) < total}, nil
}

func (l *CommunityLogic) Reply(ctx context.Context, id uint64, req ReplyRequest, key string) (int64, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return 0, err
	}
	nickname, content := actor.Card.Name, strings.TrimSpace(req.Content)
	if err := validateText("昵称", nickname, 30, true); err != nil {
		return 0, err
	}
	if err := validateText("回复", content, 500, true); err != nil {
		return 0, err
	}
	row := &model.Reply{UserID: actor.UserID, PostID: id, Nickname: nickname, Content: content, Status: "active", CreateKey: actorCreateKey(actor.UserID, key, struct {
		PostID uint64
		ReplyRequest
	}{PostID: id, ReplyRequest: req})}
	count, err := l.store.CreateReply(ctx, row)
	return count, businessError(err)
}

type ModerationRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func (l *CommunityLogic) Moderate(ctx context.Context, kind string, id uint64, req ModerationRequest) error {
	if !slicesContain([]string{"alumni", "photo", "post", "reply"}, kind) {
		return invalid("管理对象不正确")
	}
	if req.Action != "active" && req.Action != "hidden" {
		return invalid("操作只能为 active 或 hidden")
	}
	reason := strings.TrimSpace(req.Reason)
	if err := validateText("操作原因", reason, 200, true); err != nil {
		return err
	}
	return businessError(l.store.Moderate(ctx, model.AuditLog{Kind: kind, TargetID: id, Action: req.Action, Reason: reason}))
}

func (l *CommunityLogic) AuditLogs(ctx context.Context, req ContentRequest) ([]model.AuditLog, error) {
	page, size := pagination(req.Page, req.PageSize)
	return l.store.AuditLogs(ctx, dao.ContentQuery{Page: page, PageSize: size})
}

func (l *CommunityLogic) Inspect(ctx context.Context, kind string, id uint64) (*dao.ModerationItem, error) {
	item, err := l.store.Inspect(ctx, kind, id)
	return item, businessError(err)
}

var ErrImageTooLarge = errors.New("图片文件过大")

const (
	MaxUploadImageBytes = 2 << 20
	MaxStoredImageBytes = 512 << 10
	MaxImageDimension   = 2048
	MaxImagePixels      = 16000000
)

var imageSlots = make(chan struct{}, 2)

// Validate encoded bytes and dimensions before decoding; re-encode to strip metadata.
func processImage(data []byte, required bool) ([]byte, error) {
	if len(data) == 0 {
		if required {
			return nil, invalid("请选择一张照片")
		}
		return nil, nil
	}
	if len(data) > MaxUploadImageBytes {
		return nil, imageTooLarge()
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, invalid("图片内容必须为有效的 JPG 或 PNG")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return nil, invalid("图片最多 1600 万像素，请先使用一键瘦身")
	}
	select {
	case imageSlots <- struct{}{}:
		defer func() { <-imageSlots }()
	default:
		return nil, invalid("图片处理繁忙，请稍后重试")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, invalid("图片内容损坏，无法读取")
	}
	w, h := config.Width, config.Height
	if max(w, h) > MaxImageDimension {
		if w >= h {
			h = max(1, h*MaxImageDimension/w)
			w = MaxImageDimension
		} else {
			w = max(1, w*MaxImageDimension/h)
			h = MaxImageDimension
		}
	}
	for range 8 {
		target := resizeImage(source, w, h)
		for _, quality := range []int{85, 72, 60, 48} {
			var output bytes.Buffer
			if err := jpeg.Encode(&output, target, &jpeg.Options{Quality: quality}); err != nil {
				return nil, invalid("图片压缩失败")
			}
			if output.Len() <= MaxStoredImageBytes {
				return output.Bytes(), nil
			}
		}
		w, h = max(1, w*3/4), max(1, h*3/4)
	}
	return nil, invalid("图片无法压缩到 512KB，请选择其他图片")
}

func resizeImage(source image.Image, w, h int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, w, h))
	bounds := source.Bounds()
	for y := range h {
		for x := range w {
			r, g, b, a := source.At(bounds.Min.X+x*bounds.Dx()/w, bounds.Min.Y+y*bounds.Dy()/h).RGBA()
			index := target.PixOffset(x, y)
			target.Pix[index] = uint8((r + 0xffff - a) >> 8)
			target.Pix[index+1] = uint8((g + 0xffff - a) >> 8)
			target.Pix[index+2] = uint8((b + 0xffff - a) >> 8)
			target.Pix[index+3] = 255
		}
	}
	return target
}
func imageTooLarge() error {
	return fmt.Errorf("%w: 上传图片不能超过 2MB，请先使用一键瘦身（入库最多 512KB）", ErrImageTooLarge)
}
