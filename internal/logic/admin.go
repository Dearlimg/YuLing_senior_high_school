package logic

import (
	"context"
	"strings"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

type administratorKey struct{}

// WithAdministrator is only used after validating the configured bootstrap token.
func WithAdministrator(ctx context.Context) context.Context {
	return context.WithValue(ctx, administratorKey{}, true)
}

func requireAdministrator(ctx context.Context) error {
	if trusted, _ := ctx.Value(administratorKey{}).(bool); trusted {
		return nil
	}
	actor, err := CurrentActor(ctx)
	if err != nil {
		return err
	}
	if !actor.IsAdmin {
		return ErrForbidden
	}
	return nil
}

func adminAudit(ctx context.Context, kind string, id uint64, action, reason string) model.AuditLog {
	audit := model.AuditLog{Kind: kind, TargetID: id, Action: action, Reason: reason}
	if actor, err := CurrentActor(ctx); err == nil {
		audit.ActorUserID = actor.UserID
	}
	return audit
}

type AdminUsersResult struct {
	Items []dao.AdminUser `json:"items"`
	Total int64           `json:"total"`
}

type AdminRoleRequest struct {
	IsAdmin *bool  `json:"is_admin"`
	Reason  string `json:"reason"`
}

func (l *AuthLogic) AdminUsers(ctx context.Context, req ContentRequest) (*AdminUsersResult, error) {
	if err := requireAdministrator(ctx); err != nil {
		return nil, err
	}
	if err := validateText("关键词", req.Keyword, 100, false); err != nil {
		return nil, err
	}
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListUsers(ctx, dao.ContentQuery{
		Page: page, PageSize: size, Keyword: strings.TrimSpace(req.Keyword),
	})
	return &AdminUsersResult{Items: rows, Total: total}, err
}

func (l *AuthLogic) SetAdmin(ctx context.Context, id uint64, req AdminRoleRequest) error {
	if err := requireAdministrator(ctx); err != nil {
		return err
	}
	if req.IsAdmin == nil {
		return invalid("请指定管理员状态")
	}
	reason := strings.TrimSpace(req.Reason)
	if err := validateText("操作原因", reason, 200, true); err != nil {
		return err
	}
	if actor, err := CurrentActor(ctx); err == nil && actor.UserID == id && !*req.IsAdmin {
		return invalid("不能撤销自己的管理员权限，请由另一位管理员操作")
	}
	action := "grant_admin"
	if !*req.IsAdmin {
		action = "revoke_admin"
	}
	return businessError(l.store.SetUserAdmin(ctx, id, *req.IsAdmin, adminAudit(ctx, "user", id, action, reason)))
}

type ModerationResult struct {
	Items []dao.ModerationItem `json:"items"`
	Total int64                `json:"total"`
}

func (l *CommunityLogic) AdminContent(ctx context.Context, kind string, req ContentRequest) (*ModerationResult, error) {
	if err := requireAdministrator(ctx); err != nil {
		return nil, err
	}
	if !slicesContain([]string{"photo", "post", "reply", "alumni"}, kind) {
		return nil, invalid("管理对象不正确")
	}
	if req.Status != "" && !slicesContain([]string{"active", "hidden", "deleted"}, req.Status) {
		return nil, invalid("状态不正确")
	}
	if err := validateText("关键词", req.Keyword, 100, false); err != nil {
		return nil, err
	}
	page, size := pagination(req.Page, req.PageSize)
	rows, total, err := l.store.ListModeration(ctx, kind, dao.ContentQuery{
		Page: page, PageSize: size, Status: req.Status, Keyword: strings.TrimSpace(req.Keyword),
	})
	return &ModerationResult{Items: rows, Total: total}, err
}
