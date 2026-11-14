package logic

import (
	"context"
	"regexp"
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

// VisitLogic 真实访客统计：beacon 上报 + 看板聚合。
// 与 growth 的增长归因事件分离：这里只回答"多少真人看了哪些页面"。
type VisitLogic struct {
	store dao.Store
}

func NewVisitLogic(store dao.Store) *VisitLogic {
	return &VisitLogic{store: store}
}

var (
	visitPathPattern   = regexp.MustCompile(`^/[a-zA-Z0-9/_-]{0,127}$`)
	visitSourcePattern = regexp.MustCompile(`^(direct|internal|[a-z0-9][a-z0-9.-]{0,47})$`)
	visitKeyPattern    = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

type PageviewRequest struct {
	EventKey  string `json:"event_key"`
	VisitorID string `json:"visitor_id"`
	Path      string `json:"path"`
	Source    string `json:"source"`
}

// RecordPageview 校验并落库一次页面浏览。IP 与 UA 只参与哈希，不留原文。
func (l *VisitLogic) RecordPageview(ctx context.Context, req PageviewRequest, ip, ua string) error {
	if !visitKeyPattern.MatchString(req.EventKey) || !growthSlug.MatchString(req.VisitorID) {
		return invalid("访问统计标识格式不正确")
	}
	if !visitPathPattern.MatchString(req.Path) {
		return invalid("页面路径格式不正确")
	}
	if req.Source == "" {
		req.Source = "direct"
	}
	if !visitSourcePattern.MatchString(req.Source) {
		return invalid("访问来源格式不正确")
	}
	device := "desktop"
	lowerUA := strings.ToLower(ua)
	if strings.Contains(lowerUA, "mobile") || strings.Contains(lowerUA, "android") || strings.Contains(lowerUA, "iphone") {
		device = "mobile"
	}
	row := &model.VisitEvent{
		EventKey:    req.EventKey,
		VisitorHash: secretHash(req.VisitorID),
		IPHash:      secretHash(ip + "|" + ua),
		Path:        req.Path,
		Source:      req.Source,
		Device:      device,
		CreatedAt:   time.Now().UTC(),
	}
	return l.store.RecordVisit(ctx, row)
}

// Overview 看板数据。明细中的哈希只保留前缀用于分辨访客，完整哈希不出库。
func (l *VisitLogic) Overview(ctx context.Context) (*dao.VisitOverview, error) {
	out, err := l.store.VisitOverview(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	for i := range out.Recent {
		if len(out.Recent[i].VisitorHash) > 12 {
			out.Recent[i].VisitorHash = out.Recent[i].VisitorHash[:12]
		}
		out.Recent[i].IPHash = ""
		out.Recent[i].EventKey = ""
	}
	return out, nil
}
