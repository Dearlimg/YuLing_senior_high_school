package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

type OrganizerVO struct {
	ID             uint64 `json:"id"`
	AlumniID       uint64 `json:"alumni_id"`
	Name           string `json:"name"`
	GraduationYear int    `json:"graduation_year"`
	ClassName      string `json:"class_name"`
	Introduction   string `json:"introduction"`
	Status         string `json:"status"`
}

type OrganizersResult struct {
	Items []OrganizerVO `json:"items"`
	Total int64         `json:"total"`
}

type ClassHub struct {
	GraduationYear int           `json:"graduation_year"`
	ClassName      string        `json:"class_name"`
	Count          int64         `json:"count"`
	Milestone      int64         `json:"milestone"`
	NextMilestone  int64         `json:"next_milestone"`
	Organizers     []OrganizerVO `json:"organizers"`
}

func validateClass(year int, name string) error {
	if year < 1903 || year > time.Now().Year()+6 {
		return invalid("届别不正确")
	}
	if strings.TrimSpace(name) == "__unassigned__" {
		return invalid("请填写具体班级")
	}
	return validateText("班级", strings.TrimSpace(name), 30, true)
}

func organizerVO(row model.ClassOrganizer) OrganizerVO {
	return OrganizerVO{ID: row.ID, AlumniID: row.AlumniID, Name: row.Name,
		GraduationYear: row.GraduationYear, ClassName: row.ClassName,
		Introduction: row.Introduction, Status: row.Status}
}

func (l *AlumniLogic) ClassHub(ctx context.Context, req ListRequest) (*ClassHub, error) {
	name := strings.TrimSpace(req.ClassName)
	if err := validateClass(req.GraduationYear, name); err != nil {
		return nil, err
	}
	_, count, err := l.store.ListAlumni(ctx, dao.ListQuery{
		GraduationYear: req.GraduationYear, ClassName: name, Page: 1, PageSize: 1,
	})
	if err != nil {
		return nil, err
	}
	rows, _, err := l.store.ListOrganizers(ctx, dao.OrganizerQuery{
		GraduationYear: req.GraduationYear, ClassName: name, Page: 1, PageSize: 100,
	})
	if err != nil {
		return nil, err
	}
	result := &ClassHub{GraduationYear: req.GraduationYear, ClassName: name, Count: count, Organizers: []OrganizerVO{}}
	for _, n := range []int64{5, 10, 20, 30, 50, 100} {
		if count >= n {
			result.Milestone = n
			continue
		}
		result.NextMilestone = n
		break
	}
	for _, row := range rows {
		result.Organizers = append(result.Organizers, organizerVO(row))
	}
	return result, nil
}

func (l *AlumniLogic) ApplyOrganizer(ctx context.Context, intro string) error {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return err
	}
	if err := validateClass(actor.Card.GraduationYear, actor.Card.ClassName); err != nil {
		return err
	}
	intro = strings.TrimSpace(intro)
	if err := validateText("召集介绍", intro, 120, true); err != nil {
		return err
	}
	return businessError(l.store.ApplyOrganizer(ctx, &model.ClassOrganizer{
		UserID: actor.UserID, AlumniID: actor.Card.ID, GraduationYear: actor.Card.GraduationYear,
		ClassName: actor.Card.ClassName, Introduction: intro, Status: "pending",
	}))
}

func (l *AlumniLogic) Organizers(ctx context.Context, req ContentRequest, mine bool) (*OrganizersResult, error) {
	query := dao.OrganizerQuery{Private: true}
	if mine {
		actor, err := CurrentActor(ctx)
		if err != nil {
			return nil, err
		}
		query.UserID = actor.UserID
	} else if err := requireAdministrator(ctx); err != nil {
		return nil, err
	}
	query.Page, query.PageSize = pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListOrganizers(ctx, query)
	if err != nil {
		return nil, err
	}
	items := []OrganizerVO{}
	for _, row := range rows {
		items = append(items, organizerVO(row))
	}
	return &OrganizersResult{Items: items, Total: total}, nil
}

type ReviewRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (l *AlumniLogic) ReviewOrganizer(ctx context.Context, id uint64, req ReviewRequest, mine bool) error {
	review := dao.OrganizerReview{ID: id, Status: req.Status}
	if mine {
		actor, err := CurrentActor(ctx)
		if err != nil {
			return err
		}
		review.UserID, review.Status = actor.UserID, "withdrawn"
		req.Reason = "召集人本人撤回"
	} else {
		if err := requireAdministrator(ctx); err != nil {
			return err
		}
		if !slicesContain([]string{"active", "rejected", "revoked"}, req.Status) {
			return invalid("审核状态不正确")
		}
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if err := validateText("核验原因", req.Reason, 200, true); err != nil {
		return err
	}
	review.Audit = adminAudit(ctx, "organizer", id, review.Status, req.Reason)
	return businessError(l.store.ReviewOrganizer(ctx, review))
}

type RecoveryCreated struct {
	ID        uint64    `json:"id"`
	Receipt   string    `json:"receipt"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RecoveryVO struct {
	ID          uint64    `json:"id"`
	AlumniID    uint64    `json:"alumni_id"`
	Name        string    `json:"name"`
	Explanation string    `json:"explanation"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (l *AlumniLogic) RequestRecovery(ctx context.Context, id uint64, explanation string) (*RecoveryCreated, error) {
	card, err := l.store.FindAlumni(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	if card.EditCodeHash == "" {
		return nil, invalid("此卡已绑定账号，请登录个人中心管理")
	}
	explanation = strings.TrimSpace(explanation)
	if err := validateText("核验说明", explanation, 200, true); err != nil {
		return nil, err
	}
	code, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	row := &model.CardRecovery{AlumniID: id, ReceiptHash: hash, Explanation: explanation,
		Status: "pending", ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}
	if err := l.store.CreateRecovery(ctx, row); err != nil {
		return nil, businessError(err)
	}
	return &RecoveryCreated{ID: row.ID, Receipt: code, ExpiresAt: row.ExpiresAt}, nil
}

func (l *AlumniLogic) recoveryReceipt(ctx context.Context, id uint64, receipt string) (*model.CardRecovery, error) {
	row, err := l.store.FindRecovery(ctx, id)
	if err != nil {
		return nil, ErrForbidden
	}
	if !matchesSecret(receipt, row.ReceiptHash) {
		return nil, ErrForbidden
	}
	if !row.ExpiresAt.After(time.Now()) {
		return nil, invalid("申请凭证已过期，请重新申请")
	}
	return row, nil
}

func (l *AlumniLogic) RecoveryStatus(ctx context.Context, id uint64, receipt string) (string, error) {
	row, err := l.recoveryReceipt(ctx, id, receipt)
	if err != nil {
		return "", err
	}
	return row.Status, nil
}

func (l *AlumniLogic) CompleteRecovery(ctx context.Context, id uint64, receipt string) (*CreateResult, error) {
	row, err := l.recoveryReceipt(ctx, id, receipt)
	if err != nil {
		return nil, err
	}
	if row.Status != "approved" {
		return nil, invalid("申请尚未通过或已使用，请查询申请状态")
	}
	code, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	err = l.store.CompleteRecovery(ctx, dao.RecoveryCompletion{
		ID: id, AlumniID: row.AlumniID, ReceiptHash: row.ReceiptHash, NewEditHash: hash,
		Audit: adminAudit(ctx, "alumni", row.AlumniID, "reset_edit_code", fmt.Sprintf("核验申请 #%d 领取新编辑码", id)),
	})
	if err != nil {
		return nil, businessError(err)
	}
	return &CreateResult{ID: row.AlumniID, EditCode: code}, nil
}

func (l *AlumniLogic) Recoveries(ctx context.Context, req ContentRequest) ([]RecoveryVO, int64, error) {
	if err := requireAdministrator(ctx); err != nil {
		return nil, 0, err
	}
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListRecoveries(ctx, dao.ContentQuery{Page: page, PageSize: size})
	if err != nil {
		return nil, 0, err
	}
	items := []RecoveryVO{}
	for _, row := range rows {
		status := row.Status
		if !row.ExpiresAt.After(time.Now()) && (status == "pending" || status == "approved") {
			status = "expired"
		}
		items = append(items, RecoveryVO{ID: row.ID, AlumniID: row.AlumniID, Name: row.Name, Explanation: row.Explanation,
			Status: status, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt})
	}
	return items, total, nil
}

func (l *AlumniLogic) ReviewRecovery(ctx context.Context, id uint64, req ReviewRequest) error {
	if err := requireAdministrator(ctx); err != nil {
		return err
	}
	if req.Status != "approved" && req.Status != "rejected" {
		return invalid("审核状态不正确")
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if err := validateText("身份核验记录", req.Reason, 200, true); err != nil {
		return err
	}
	return businessError(l.store.ReviewRecovery(ctx, dao.RecoveryReview{ID: id, Status: req.Status,
		Audit: adminAudit(ctx, "recovery", id, req.Status, req.Reason)}))
}

func (l *AlumniLogic) CohortUniversities(ctx context.Context, year int) ([]dao.University, int64, error) {
	if year < 1903 || year > time.Now().Year()+6 {
		return nil, 0, invalid("届别不正确")
	}
	rows, err := l.store.CohortUniversities(ctx, year)
	var total int64
	for _, row := range rows {
		total += row.Count
	}
	return rows, total, err
}
