// Package logic 业务逻辑层。
//
// 边界约定：
//   - 只允许通过 dao 访问数据，不直接持有 gorm.DB / redis.Client 做查询。
//   - 承担参数语义校验、领域规则、DTO 装配。
//   - 不感知 gin，输入输出都是普通结构体，方便单测脱离 HTTP 跑。
package logic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

// 业务级错误，由 controller 翻译成对应的 HTTP 状态码。
var (
	ErrInvalidParam = errors.New("参数不合法")
	ErrNotFound     = errors.New("资源不存在")
)

var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// AlumniLogic 校友业务。
type AlumniLogic struct {
	alumniDAO *dao.AlumniDAO
}

// NewAlumniLogic 构造。
func NewAlumniLogic(alumniDAO *dao.AlumniDAO) *AlumniLogic {
	return &AlumniLogic{alumniDAO: alumniDAO}
}

// ListRequest 列表入参。
type ListRequest struct {
	GraduationYear int    `form:"graduation_year" json:"graduation_year"`
	City           string `form:"city" json:"city"`
	Keyword        string `form:"keyword" json:"keyword"`
	Page           int    `form:"page" json:"page"`
	PageSize       int    `form:"page_size" json:"page_size"`
}

// AlumniVO 对外视图。
// 单独定义 VO 而不是直接返回 model，是为了把"存什么"和"给前端看什么"分开：
// 以后要加字段级权限（比如手机号仅校友可见），只改这里即可。
type AlumniVO struct {
	ID             uint64 `json:"id"`
	Name           string `json:"name"`
	Gender         int8   `json:"gender"`
	GraduationYear int    `json:"graduation_year"`
	ClassName      string `json:"class_name"`
	City           string `json:"city"`
	Industry       string `json:"industry"`
	Company        string `json:"company"`
	Position       string `json:"position"`
	Status         string `json:"status"`
}

// ListResult 分页结果。
type ListResult struct {
	Items    []AlumniVO `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// List 分页查询校友。
func (l *AlumniLogic) List(ctx context.Context, req ListRequest) (*ListResult, error) {
	// 分页参数做安全上限，避免一次拉全表把内存打满。
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	rows, total, err := l.alumniDAO.List(ctx, dao.ListQuery{
		GraduationYear: req.GraduationYear,
		City:           strings.TrimSpace(req.City),
		Keyword:        strings.TrimSpace(req.Keyword),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("查询校友列表失败: %w", err)
	}

	items := make([]AlumniVO, 0, len(rows))
	for _, r := range rows {
		items = append(items, toVO(r))
	}
	return &ListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetByID 查询单条。
func (l *AlumniLogic) GetByID(ctx context.Context, id uint64) (*AlumniVO, error) {
	row, err := l.alumniDAO.GetByID(ctx, id)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询校友详情失败: %w", err)
	}
	vo := toVO(*row)
	return &vo, nil
}

// CreateRequest 创建入参。
type CreateRequest struct {
	Name           string `json:"name" binding:"required"`
	Gender         int8   `json:"gender"`
	GraduationYear int    `json:"graduation_year" binding:"required"`
	ClassName      string `json:"class_name"`
	City           string `json:"city"`
	Industry       string `json:"industry"`
	Company        string `json:"company"`
	Position       string `json:"position"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	Remark         string `json:"remark"`
}

// Create 新增校友档案。
func (l *AlumniLogic) Create(ctx context.Context, req CreateRequest) (uint64, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return 0, fmt.Errorf("%w: 姓名不能为空", ErrInvalidParam)
	}
	// 毕业年份限定在合理区间，防止脏数据污染按届检索。
	currentYear := time.Now().Year()
	if req.GraduationYear < 1950 || req.GraduationYear > currentYear {
		return 0, fmt.Errorf("%w: 毕业年份需在 1950~%d 之间", ErrInvalidParam, currentYear)
	}
	if req.Phone != "" && !phonePattern.MatchString(req.Phone) {
		return 0, fmt.Errorf("%w: 手机号格式不正确", ErrInvalidParam)
	}

	row := &model.Alumni{
		Name:           name,
		Gender:         req.Gender,
		GraduationYear: req.GraduationYear,
		ClassName:      strings.TrimSpace(req.ClassName),
		City:           strings.TrimSpace(req.City),
		Industry:       strings.TrimSpace(req.Industry),
		Company:        strings.TrimSpace(req.Company),
		Position:       strings.TrimSpace(req.Position),
		Phone:          strings.TrimSpace(req.Phone),
		Email:          strings.TrimSpace(req.Email),
		Remark:         strings.TrimSpace(req.Remark),
	}
	if err := l.alumniDAO.Create(ctx, row); err != nil {
		return 0, fmt.Errorf("写入校友档案失败: %w", err)
	}
	return row.ID, nil
}

// Count 统计有效校友数。
func (l *AlumniLogic) Count(ctx context.Context) (int64, error) {
	return l.alumniDAO.Count(ctx)
}

func toVO(m model.Alumni) AlumniVO {
	return AlumniVO{
		ID:             m.ID,
		Name:           m.Name,
		Gender:         m.Gender,
		GraduationYear: m.GraduationYear,
		ClassName:      m.ClassName,
		City:           m.City,
		Industry:       m.Industry,
		Company:        m.Company,
		Position:       m.Position,
		Status:         "active",
	}
}
