package logic

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

var growthSlug = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)

type Attribution struct {
	VisitorID      string `json:"visitor_id"`
	Source         string `json:"source"`
	Campaign       string `json:"campaign"`
	ShareID        string `json:"share_id"`
	GraduationYear int    `json:"graduation_year"`
	ClassName      string `json:"class_name"`
}

type ClaimRequest struct {
	CreateRequest
	Attribution Attribution `json:"attribution"`
}

type GrowthRequest struct {
	Attribution
	Event string `json:"event"`
}

func growthEvent(a Attribution, event string) (*model.GrowthEvent, error) {
	if !growthSlug.MatchString(a.VisitorID) {
		return nil, invalid("匿名访问标识格式不正确")
	}
	for _, value := range []string{a.Source, a.Campaign, a.ShareID} {
		if value != "" && !growthSlug.MatchString(value) {
			return nil, invalid("来源参数仅支持 48 位以内的字母、数字、下划线和短横线")
		}
	}
	if a.Source == "" {
		a.Source = "direct"
	}
	if a.GraduationYear != 0 && (a.GraduationYear < 1903 || a.GraduationYear > time.Now().Year()+6) {
		return nil, invalid("届别参数不正确")
	}
	a.ClassName = strings.TrimSpace(a.ClassName)
	if err := validateText("班级", a.ClassName, 30, false); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	visitor := secretHash(a.VisitorID)
	key := fmt.Sprintf(
		"%s:%s:%s:%s:%s:%s:%d:%s",
		now.Format("2006-01-02"),
		event,
		visitor,
		a.ShareID,
		a.Source,
		a.Campaign,
		a.GraduationYear,
		a.ClassName,
	)
	return &model.GrowthEvent{
		EventKey: secretHash(key), Event: event, VisitorHash: visitor,
		Source: a.Source, Campaign: a.Campaign, ShareID: a.ShareID,
		GraduationYear: a.GraduationYear, ClassName: a.ClassName, CreatedAt: now,
	}, nil
}

func (l *AlumniLogic) Claim(ctx context.Context, req ClaimRequest, key string) (*CreateResult, error) {
	row, err := validateAlumni(req.CreateRequest)
	if err != nil {
		return nil, err
	}
	a := req.Attribution
	if a.VisitorID == "" {
		a.VisitorID = rand.Text()
	}
	a.GraduationYear, a.ClassName = row.GraduationYear, row.ClassName
	event, err := growthEvent(a, "registration_success")
	if err != nil {
		return nil, err
	}
	code, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	row.EditCodeHash = hash
	row.CreateKey = createKey(key, req.CreateRequest)
	event.EventKey = secretHash(event.EventKey + ":" + *row.CreateKey)
	if err := l.store.ClaimAlumni(ctx, row, event); err != nil {
		return nil, businessError(err)
	}
	return &CreateResult{ID: row.ID, EditCode: code}, nil
}

func (l *AlumniLogic) RecordGrowth(ctx context.Context, req GrowthRequest) error {
	switch req.Event {
	case "registration_view", "registration_start", "registration_failure", "share_click",
		"poster_save", "poster_view", "share_visit", "edit_code_copy":
	default:
		return invalid("不支持的统计事件")
	}
	if req.Event == "share_visit" && req.ShareID == "" {
		return invalid("分享回访需要分享标识")
	}
	row, err := growthEvent(req.Attribution, req.Event)
	if err != nil {
		return err
	}
	return l.store.RecordGrowth(ctx, row)
}

type GrowthMetricsRequest struct {
	Days           int    `form:"days"`
	Source         string `form:"source"`
	Campaign       string `form:"campaign"`
	GraduationYear int    `form:"graduation_year"`
	ClassName      string `form:"class_name"`
}

type GrowthMetrics struct {
	Days              int              `json:"days"`
	Counts            map[string]int64 `json:"counts"`
	RegistrationRate  *float64         `json:"registration_rate"`
	CompletionRate    *float64         `json:"completion_rate"`
	ShareRate         *float64         `json:"share_rate"`
	ReturnRate        *float64         `json:"return_rate"`
	ShareRegisterRate *float64         `json:"share_registration_rate"`
}

func (l *AlumniLogic) GrowthMetrics(ctx context.Context, req GrowthMetricsRequest) (*GrowthMetrics, error) {
	if req.Days == 0 {
		req.Days = 30
	}
	if req.Days < 1 || req.Days > 90 {
		return nil, invalid("统计范围为 1～90 天")
	}
	if _, err := growthEvent(Attribution{
		VisitorID: "validation", Source: req.Source, Campaign: req.Campaign,
		GraduationYear: req.GraduationYear, ClassName: req.ClassName,
	}, "validation"); err != nil {
		return nil, err
	}
	rows, err := l.store.GrowthCounts(ctx, dao.GrowthQuery{
		Since: time.Now().UTC().Add(-time.Duration(req.Days) * 24 * time.Hour), Source: req.Source,
		Campaign: req.Campaign, GraduationYear: req.GraduationYear, ClassName: strings.TrimSpace(req.ClassName),
	})
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Event] = row.UV
		if row.Event == "registration_success" {
			counts["share_registration"] = row.ShareUV
		}
	}
	return &GrowthMetrics{
		Days: req.Days, Counts: counts,
		RegistrationRate:  ratio(counts["registration_success"], counts["registration_view"]),
		CompletionRate:    ratio(counts["registration_success"], counts["registration_start"]),
		ShareRate:         ratio(counts["share"], counts["registration_success"]),
		ReturnRate:        ratio(counts["share_visit"], counts["share"]),
		ShareRegisterRate: ratio(counts["share_registration"], counts["share_visit"]),
	}, nil
}

func ratio(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}
