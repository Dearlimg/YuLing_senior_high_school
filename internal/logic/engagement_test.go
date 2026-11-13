package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

func TestOrganizerApprovalMilestonesAndCohortPrivacy(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l, auth := NewAlumniLogic(store), NewAuthLogic(store, store)
	account, err := auth.Register(ctx, RegisterRequest{Account: "organizer", Password: "password123",
		CreateRequest: CreateRequest{Name: "班级志愿者", GraduationYear: 2018, ClassName: "高三1班", University: "西安交大"}}, "one")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := auth.Authenticate(ctx, account.Token)
	if err != nil {
		t.Fatal(err)
	}
	owner := WithActor(ctx, actor)
	for i := range 4 {
		_, err := l.Create(ctx, CreateRequest{Name: "同窗", GraduationYear: 2018, ClassName: "高三1班"}, string(rune('a'+i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = l.Create(ctx, CreateRequest{Name: "在读", Identity: "student", GraduationYear: time.Now().Year() + 1,
		ClassName: "高三1班", DreamUniversity: "西安交大"}, "student")
	if err != nil {
		t.Fatal(err)
	}
	if err = l.ApplyOrganizer(owner, "邀请同班同学回来看看"); err != nil {
		t.Fatal(err)
	}
	hub, err := l.ClassHub(ctx, ListRequest{GraduationYear: 2018, ClassName: "高三1班"})
	if err != nil {
		t.Fatal(err)
	}
	if hub.Count != 5 || hub.Milestone != 5 || hub.NextMilestone != 10 {
		t.Fatalf("milestones: %#v", hub)
	}
	if len(hub.Organizers) != 0 {
		t.Fatal("pending organizer was public")
	}
	if err = l.ReviewOrganizer(ctx, 1, ReviewRequest{Status: "active", Reason: "已核对班级"}, false); err == nil {
		t.Fatal("public review accepted")
	}
	if err = l.ReviewOrganizer(WithAdministrator(ctx), 1, ReviewRequest{Status: "active", Reason: "已核对班级"}, false); err != nil {
		t.Fatal(err)
	}
	hub, _ = l.ClassHub(ctx, ListRequest{GraduationYear: 2018, ClassName: "高三1班"})
	if len(hub.Organizers) != 1 {
		t.Fatal("approved organizer missing")
	}
	encoded, _ := json.Marshal(hub)
	if strings.Contains(string(encoded), "user_id") || strings.Contains(string(encoded), "phone") {
		t.Fatal("public organizer leak")
	}
	rows, total, err := l.CohortUniversities(ctx, 2018)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("cohort: %#v %d %v", rows, total, err)
	}
	if rows[0].Name != "西安交通大学" {
		t.Fatal("alias not normalized")
	}
	_, err = auth.UpdateCard(owner, CreateRequest{Name: actor.Card.Name, GraduationYear: 2018, ClassName: "高三2班"})
	if err != nil {
		t.Fatal(err)
	}
	hub, _ = l.ClassHub(ctx, ListRequest{GraduationYear: 2018, ClassName: "高三1班"})
	if len(hub.Organizers) != 0 {
		t.Fatal("organizer remained in old class")
	}
}

func TestRecoveryRequiresReviewAndConsumesReceipt(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l := NewAlumniLogic(store)
	card, err := l.Create(ctx, CreateRequest{Name: "找回同学", GraduationYear: 2018}, "card")
	if err != nil {
		t.Fatal(err)
	}
	first, err := l.RequestRecovery(ctx, card.ID, "可由熟识的班主任协助核实")
	if err != nil {
		t.Fatal(err)
	}
	second, err := l.RequestRecovery(ctx, card.ID, "另一份申请")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.RecoveryStatus(ctx, first.ID, "wrong"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err = l.CompleteRecovery(ctx, first.ID, first.Receipt); !errors.Is(err, ErrInvalidParam) {
		t.Fatal("unreviewed reset accepted")
	}
	if err = l.ReviewRecovery(ctx, first.ID, ReviewRequest{Status: "approved", Reason: "核验"}); err == nil {
		t.Fatal("unauthorized review")
	}
	admin := WithAdministrator(ctx)
	if err = l.ReviewRecovery(admin, first.ID, ReviewRequest{Status: "approved", Reason: "已通过熟识班主任核验申请本人及编号"}); err != nil {
		t.Fatal(err)
	}
	reset, err := l.CompleteRecovery(ctx, first.ID, first.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Unlock(ctx, card.ID, card.EditCode); !errors.Is(err, ErrForbidden) {
		t.Fatal("old code still valid")
	}
	if _, err = l.Unlock(ctx, card.ID, reset.EditCode); err != nil {
		t.Fatal("new code not usable")
	}
	if _, err = l.CompleteRecovery(ctx, first.ID, first.Receipt); err == nil {
		t.Fatal("receipt reused")
	}
	status, err := l.RecoveryStatus(ctx, second.ID, second.Receipt)
	if err != nil || status != "revoked" {
		t.Fatalf("other request not revoked: %s %v", status, err)
	}
	requests, _, err := l.Recoveries(admin, ContentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(requests)
	if strings.Contains(string(data), "receipt") || strings.Contains(string(data), first.Receipt) {
		t.Fatal("admin list leaked receipt")
	}
}

func TestRecoveryExpiryAndAccountBinding(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l := NewAlumniLogic(store)
	card, err := l.Create(ctx, CreateRequest{Name: "过期测试", GraduationYear: 2018}, "expiry")
	if err != nil {
		t.Fatal(err)
	}
	expired := &model.CardRecovery{AlumniID: card.ID, ReceiptHash: secretHash("expired-receipt"), Status: "approved", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := store.CreateRecovery(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CompleteRecovery(ctx, expired.ID, "expired-receipt"); err == nil {
		t.Fatal("expired receipt accepted")
	}
	req, err := l.RequestRecovery(ctx, card.ID, "本人核验")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ReviewRecovery(WithAdministrator(ctx), req.ID, ReviewRequest{Status: "approved", Reason: "已核实"}); err != nil {
		t.Fatal(err)
	}
	auth := NewAuthLogic(store, store)
	_, err = auth.Register(ctx, RegisterRequest{Account: "bound_card", Password: "password123", AlumniID: card.ID,
		CreateRequest: CreateRequest{EditCode: card.EditCode}}, "bind")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CompleteRecovery(ctx, req.ID, req.Receipt); err == nil {
		t.Fatal("reset bound card")
	}
}
