package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"YuLing_senior_high_school/internal/dao"
)

func TestClaimPrivacyAttributionAndDuplicate(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l := NewAlumniLogic(store)
	req := ClaimRequest{
		CreateRequest: CreateRequest{Name: "增长同学", GraduationYear: 2018, ClassName: "高三1班"},
		Attribution:   Attribution{VisitorID: "browser_a", Source: "class_invite", Campaign: "reunion", ShareID: "invite_a"},
	}
	created, err := l.Claim(ctx, req, "claim-one")
	if err != nil || len(created.EditCode) != 32 {
		t.Fatalf("claim: %#v %v", created, err)
	}
	owner, err := l.Unlock(ctx, created.ID, created.EditCode)
	if err != nil || owner.ContactPublic || owner.PhonePublic || owner.WechatPublic {
		t.Fatalf("private defaults: %#v %v", owner, err)
	}
	if _, err := l.Claim(ctx, req, "claim-one"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := l.Claim(ctx, req, "claim-two"); err != nil {
		t.Fatalf("distinct claim key: %v", err)
	}
	events := []string{
		"registration_view", "registration_start", "share_visit", "share_click", "poster_save", "share_click",
	}
	for _, event := range events {
		if err := l.RecordGrowth(ctx, GrowthRequest{Attribution: req.Attribution, Event: event}); err != nil {
			t.Fatal(err)
		}
	}
	metrics, err := l.GrowthMetrics(ctx, GrowthMetricsRequest{Source: "class_invite", Campaign: "reunion"})
	if err != nil {
		t.Fatal(err)
	}
	validCounts := metrics.Counts["registration_success"] == 1 &&
		metrics.Counts["share"] == 1 && metrics.Counts["share_registration"] == 1
	if !validCounts {
		t.Fatalf("UV counts: %#v %v", metrics, err)
	}
	if metrics.ShareRegisterRate == nil || *metrics.ShareRegisterRate != 1 {
		t.Fatalf("share conversion: %#v", metrics)
	}
	other, err := l.GrowthMetrics(ctx, GrowthMetricsRequest{Campaign: "other"})
	if err != nil || len(other.Counts) != 0 || other.RegistrationRate != nil {
		t.Fatalf("filter: %#v %v", other, err)
	}
	public, err := l.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(public)
	for _, forbidden := range []string{"visitor", "share_id", "edit_code", "phone", "wechat"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("public leak: %s", encoded)
		}
	}
}

func TestGrowthRejectsForgedSuccessAndInvalidAttribution(t *testing.T) {
	l := NewAlumniLogic(dao.NewMemoryStore())
	for _, req := range []GrowthRequest{
		{Attribution: Attribution{VisitorID: "browser"}, Event: "registration_success"},
		{Attribution: Attribution{VisitorID: "browser"}, Event: "share_visit"},
		{Attribution: Attribution{VisitorID: "browser", Campaign: "private@example.com"}, Event: "registration_view"},
		{Attribution: Attribution{VisitorID: "browser", GraduationYear: 1}, Event: "registration_view"},
	} {
		if err := l.RecordGrowth(context.Background(), req); !errors.Is(err, ErrInvalidParam) {
			t.Fatalf("accepted %#v: %v", req, err)
		}
	}
	if _, err := l.GrowthMetrics(context.Background(), GrowthMetricsRequest{Days: 100}); !errors.Is(err, ErrInvalidParam) {
		t.Fatal(err)
	}
}

func TestClaimCanBeBoundWithoutCreatingAnotherCard(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l := NewAlumniLogic(store)
	card, err := l.Claim(ctx, ClaimRequest{CreateRequest: CreateRequest{Name: "可选账号", GraduationYear: 2018}}, "bind")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := l.Unlock(ctx, card.ID, card.EditCode)
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Update(ctx, card.ID, CreateRequest{
		Name: owner.Name, GraduationYear: owner.GraduationYear, EditCode: card.EditCode,
		Phone: "13800138000", Wechat: "private_chat", PhonePublic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, _ = l.Unlock(ctx, card.ID, card.EditCode)
	if !owner.PhonePublic || owner.WechatPublic {
		t.Fatalf("per-field privacy: %#v", owner)
	}
	auth := NewAuthLogic(store, store)
	result, err := auth.Register(ctx, RegisterRequest{
		CreateRequest: CreateRequest{EditCode: card.EditCode},
		Account:       "growth_user", Password: "password123", AlumniID: card.ID,
	}, "bind-account")
	if err != nil || result.ID != card.ID {
		t.Fatalf("bind: %#v %v", result, err)
	}
	if _, err := l.Unlock(ctx, card.ID, card.EditCode); !errors.Is(err, ErrForbidden) {
		t.Fatalf("bound code still usable: %v", err)
	}
	meta, _ := l.Meta(ctx)
	if meta.Total != 1 {
		t.Fatalf("binding created extra card: %#v", meta)
	}
}
