package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

func TestRegistrationPrivacyEditingAndUniversityConsistency(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	service := NewAlumniLogic(store)
	req := CreateRequest{
		Name: " 测试校友 ", GraduationYear: 2018, ClassName: "高三1班",
		University: " 西安交大 ", Major: "电气工程", City: "西安", Company: "测试单位",
		Phone: "13800138000", Wechat: "private-wechat", Email: "private@example.com",
		ContactPublic: true,
	}
	created, err := service.Create(ctx, req, "registration-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(created.EditCode) < 32 {
		t.Fatal("weak edit code")
	}
	row, _ := store.FindAlumni(ctx, created.ID)
	if row.EditCodeHash == created.EditCode || row.EditCodeHash == "" {
		t.Fatal("secret must only be stored as a hash")
	}
	if _, err := service.Create(ctx, req, "registration-key"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	public, _ := service.GetByID(ctx, created.ID)
	encoded, _ := json.Marshal(public)
	for _, private := range []string{"13800138000", "private-wechat", "private@example.com", created.EditCode, "edit_code", "phone", "email", "wechat"} {
		if bytes.Contains(encoded, []byte(private)) {
			t.Fatalf("public data leaked %s", private)
		}
	}
	if public.University != "西安交通大学" || public.Name != "测试校友" {
		t.Fatal("normalization failed")
	}
	for _, keyword := range []string{"测试校友", "交通", "电气", "西安", "测试单位"} {
		result, err := service.List(ctx, ListRequest{Keyword: keyword, GraduationYear: 2018})
		if err != nil || result.Total != 1 {
			t.Fatalf("search %s: %#v %v", keyword, result, err)
		}
	}
	if _, err := service.Unlock(ctx, created.ID, "wrong"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong secret accepted: %v", err)
	}
	owner, err := service.Unlock(ctx, created.ID, created.EditCode)
	if err != nil || owner.Phone != req.Phone {
		t.Fatal("owner unlock failed")
	}
	req.University, req.Message, req.EditCode = "北大", "你好，同窗", created.EditCode
	if _, err := service.Update(ctx, created.ID, req); err != nil {
		t.Fatal(err)
	}
	unis, total, _ := service.Universities(ctx)
	if total != 1 || len(unis) != 1 || unis[0].Name != "北京大学" {
		t.Fatalf("university update out of sync: %#v", unis)
	}
	result, _ := service.List(ctx, ListRequest{University: "北大"})
	if result.Total != unis[0].Count {
		t.Fatal("ranking and directory counts differ")
	}
}

func TestStudentDreamSchoolExcludedUntilExplicitGraduation(t *testing.T) {
	ctx := context.Background()
	service := NewAlumniLogic(dao.NewMemoryStore())
	req := CreateRequest{
		Name: "在读测试", Identity: "student", GraduationYear: time.Now().Year() + 1,
		ClassName: "高三1班", DreamUniversity: "北大", University: "清华",
	}
	created, err := service.Create(ctx, req, "student")
	if err != nil {
		t.Fatal(err)
	}
	person, _ := service.GetByID(ctx, created.ID)
	if person.University != "" || person.DreamUniversity != "北京大学" {
		t.Fatal("actual and dream schools mixed")
	}
	unis, total, _ := service.Universities(ctx)
	if total != 0 || len(unis) != 0 {
		t.Fatal("student polluted actual university statistics")
	}
	list, _ := service.List(ctx, ListRequest{University: "北京大学"})
	if list.Total != 0 {
		t.Fatal("university reverse lookup included a dream school")
	}
	req.Identity, req.GraduationYear, req.University, req.EditCode = "alumni", 2026, "北大", created.EditCode
	if _, err := service.Update(ctx, created.ID, req); err != nil {
		t.Fatal(err)
	}
	_, total, _ = service.Universities(ctx)
	if total != 1 {
		t.Fatal("explicit graduate conversion failed")
	}
}

func TestAlumniValidation(t *testing.T) {
	base := CreateRequest{Name: "校友", GraduationYear: 2018}
	tests := []struct {
		name   string
		change func(*CreateRequest)
	}{
		{name: "empty name", change: func(r *CreateRequest) { r.Name = " " }},
		{name: "future graduate", change: func(r *CreateRequest) { r.GraduationYear = 9999 }},
		{name: "invalid phone", change: func(r *CreateRequest) { r.Phone = "123" }},
		{name: "invalid email", change: func(r *CreateRequest) { r.Email = "hello" }},
		{name: "long message", change: func(r *CreateRequest) { r.Message = strings.Repeat("榆", 121) }},
		{name: "student class", change: func(r *CreateRequest) { r.Identity = "student"; r.GraduationYear = 2027 }},
		{name: "invalid education", change: func(r *CreateRequest) { r.Education = "fake" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.change(&req)
			if _, err := validateAlumni(req); !errors.Is(err, ErrInvalidParam) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 30, 20))
	img.Set(2, 2, color.RGBA{R: 180, G: 40, B: 50, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestPhotoCodesAndModerationHideImages(t *testing.T) {
	ctx := testActorContext()
	store := dao.NewMemoryStore()
	service := NewCommunityLogic(store)
	created, err := service.Upload(ctx, PhotoRequest{Caption: "校园", Uploader: "校友", Category: "校园风景", Image: testPNG(t), Key: "photo"})
	if err != nil {
		t.Fatal(err)
	}
	data, mime, err := service.PhotoImage(ctx, created.ID)
	if err != nil || mime != "image/jpeg" || len(data) == 0 {
		t.Fatal("image not re-encoded")
	}
	row, _ := store.FindPhoto(ctx, created.ID)
	if row.DeleteCodeHash == created.DeleteCode {
		t.Fatal("raw delete code stored")
	}
	if err := service.DeletePhoto(WithActor(ctx, &Actor{UserID: 2}), created.ID, "wrong"); !errors.Is(err, ErrForbidden) {
		t.Fatal("wrong delete code accepted")
	}
	if err := service.Moderate(ctx, "photo", created.ID, ModerationRequest{Action: "hidden", Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.PhotoImage(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("hidden image still publicly accessible")
	}
	if err := service.Moderate(ctx, "photo", created.ID, ModerationRequest{Action: "active", Reason: "restore"}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePhoto(ctx, created.ID, created.DeleteCode); err != nil {
		t.Fatal(err)
	}
	photos, _ := service.Photos(ctx, ContentRequest{})
	if photos.Total != 0 {
		t.Fatal("deleted photo still in album")
	}
	if _, _, err := service.PhotoImage(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted image still accessible")
	}
	if err := service.Moderate(ctx, "photo", created.ID, ModerationRequest{Action: "active", Reason: "restore"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("owner-deleted photo restored by moderation")
	}
}

func TestPostsRepliesAndParentModeration(t *testing.T) {
	ctx := testActorContext()
	store := dao.NewMemoryStore()
	service := NewCommunityLogic(store)
	id, err := service.CreatePost(ctx, PostRequest{Nickname: "校友", Content: "你好", Key: "post"})
	if err != nil {
		t.Fatal(err)
	}
	count, err := service.Reply(ctx, id, ReplyRequest{Nickname: "同窗", Content: "回复1"}, "r1")
	if err != nil || count != 1 {
		t.Fatal("reply count failed")
	}
	if _, err := service.Reply(ctx, id, ReplyRequest{Nickname: "同窗", Content: "回复1"}, "r1"); !errors.Is(err, ErrDuplicate) {
		t.Fatal("duplicate reply accepted")
	}
	if _, err := service.Reply(ctx, id, ReplyRequest{Nickname: "同窗", Content: " "}, "blank"); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("empty reply accepted")
	}
	_, _ = service.Reply(ctx, id, ReplyRequest{Nickname: "同窗", Content: "回复2"}, "r2")
	result, _ := service.Replies(ctx, id, ContentRequest{PageSize: 1})
	if result.Total != 2 || !result.HasMore || result.Items[0].Content != "回复1" {
		t.Fatal("reply order or pagination failed")
	}
	posts, _ := service.Posts(ctx, ContentRequest{})
	if posts.Items[0].ReplyCount != 2 {
		t.Fatal("reply count stale")
	}
	_ = service.Moderate(ctx, "post", id, ModerationRequest{Action: "hidden", Reason: "test"})
	if _, err := service.Reply(ctx, id, ReplyRequest{Nickname: "同窗", Content: "不能回复"}, "r3"); !errors.Is(err, ErrNotFound) {
		t.Fatal("hidden parent accepted reply")
	}
	if _, err := service.Replies(ctx, id, ContentRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatal("hidden parent replies still public")
	}
}

func TestImageValidation(t *testing.T) {
	for _, data := range [][]byte{[]byte("<script>alert(1)</script>"), bytes.Repeat([]byte("x"), (10<<20)+1)} {
		if _, err := processImage(data, true); !errors.Is(err, ErrInvalidParam) && !errors.Is(err, ErrImageTooLarge) {
			t.Fatal("invalid image accepted")
		}
	}
}

func testActorContext() context.Context {
	return WithActor(context.Background(), &Actor{IsAdmin: true, UserID: 1, Card: model.Alumni{ID: 1, Name: "测试校友", GraduationYear: 2018, AvatarColor: "blue"}})
}
