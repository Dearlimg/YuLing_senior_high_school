package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"YuLing_senior_high_school/internal/dao"
)

func TestAccountOwnershipLegacyClaimAndLogout(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	auth := NewAuthLogic(store, store)
	req := RegisterRequest{Account: " Test_Account ", Password: "password123",
		CreateRequest: CreateRequest{Name: "校友", GraduationYear: 2018, Phone: "13800138000"}}
	saved, err := auth.Register(ctx, req, "register")
	if err != nil {
		t.Fatal(err)
	}
	user, _ := store.FindUser(ctx, "test_account")
	if user.PasswordHash == req.Password || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		t.Fatal("password must be stored with bcrypt")
	}
	if _, err := auth.Register(ctx, req, "another"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate account: %v", err)
	}
	stats, _ := store.Stats(ctx)
	if stats.Alumni != 1 {
		t.Fatal("duplicate registration left an orphan card")
	}
	if _, err := auth.Login(ctx, LoginRequest{Account: "test_account", Password: "wrong"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("wrong password accepted")
	}
	actor, err := auth.Authenticate(ctx, saved.Token)
	if err != nil {
		t.Fatal(err)
	}
	owner := WithActor(ctx, actor)
	if _, err := auth.UpdateCard(owner, CreateRequest{Name: "新名字", GraduationYear: 2018}); err != nil {
		t.Fatal(err)
	}
	card, _ := store.FindAlumni(ctx, saved.ID)
	if card.Name != "新名字" {
		t.Fatal("owner update failed")
	}
	legacy := NewAlumniLogic(store)
	old, err := legacy.Create(ctx, CreateRequest{Name: "旧卡", GraduationYear: 2015}, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	claim := RegisterRequest{Account: "legacy_user", Password: "password123", AlumniID: old.ID,
		CreateRequest: CreateRequest{EditCode: old.EditCode}}
	if _, err := auth.Register(ctx, claim, "claim"); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Unlock(ctx, old.ID, old.EditCode); !errors.Is(err, ErrForbidden) {
		t.Fatal("claimed legacy edit code still works")
	}
	claim.Account = "second_claim"
	if _, err := auth.Register(ctx, claim, "claim2"); err == nil {
		t.Fatal("card claimed twice")
	}
	if err := auth.Logout(ctx, saved.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(ctx, saved.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("revoked session accepted")
	}
	if err := store.SaveSession(ctx, secretHash(saved.Token), user.ID, -time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(ctx, saved.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("expired session accepted")
	}
}

func TestCommunityRequiresIdentityAndIgnoresSpoofedAuthor(t *testing.T) {
	store := dao.NewMemoryStore()
	service := NewCommunityLogic(store)
	if _, err := service.CreatePost(context.Background(), PostRequest{Nickname: "fake", Content: "hello"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("anonymous post accepted")
	}
	id, err := service.CreatePost(testActorContext(), PostRequest{Nickname: "fake", GraduationYear: 2000, Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	post, _ := store.FindPost(context.Background(), id)
	if post.UserID != 1 || post.Nickname != "测试校友" || post.GraduationYear != 2018 {
		t.Fatal("request forged author identity")
	}
}
