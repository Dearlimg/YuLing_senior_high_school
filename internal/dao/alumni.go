// Package dao 数据访问层。
//
// 边界约定：
//   - 只有本层可以直接使用 gorm.DB / redis.Client。
//   - 本层不返回 HTTP 语义，也不做业务判断（"该不该允许"属于 logic）。
//   - 本层方法第一个参数固定为 context.Context，便于上层控制超时与取消。
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"YuLing_senior_high_school/internal/model"
)

// ErrNotFound 统一表达"查无此记录"，让 logic 层不必依赖 gorm.ErrRecordNotFound。
// 直接用 gorm 的错误会让业务层间接耦合 ORM，换实现时全线改。
var ErrNotFound = errors.New("记录不存在")

// AlumniDAO 校友数据访问。
type AlumniDAO struct {
	db *gorm.DB
}

// NewAlumniDAO 构造。
func NewAlumniDAO(db *gorm.DB) *AlumniDAO { return &AlumniDAO{db: db} }

// ListQuery 列表查询条件。放在 dao 层而非 controller 层，
// 是为了让分页、过滤的语义在数据层一次定死，controller 只负责搬运参数。
type ListQuery struct {
	GraduationYear int
	City           string
	Keyword        string
	Page           int
	PageSize       int
}

// List 按条件分页查询，返回记录与总数。
func (d *AlumniDAO) List(ctx context.Context, q ListQuery) ([]model.Alumni, int64, error) {
	tx := d.db.WithContext(ctx).Model(&model.Alumni{}).Where("deleted_at IS NULL")

	if q.GraduationYear > 0 {
		tx = tx.Where("graduation_year = ?", q.GraduationYear)
	}
	if q.City != "" {
		tx = tx.Where("city = ?", q.City)
	}
	if q.Keyword != "" {
		// 这里用前缀匹配而非 %keyword%，是为了让 name 上的索引还能部分生效。
		// 数据量上来后要换成全文索引或搜索引擎，注释留在这里提醒后来者。
		tx = tx.Where("name LIKE ?", q.Keyword+"%")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.Alumni
	err := tx.Order("graduation_year DESC, id DESC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetByID 按主键查询，未命中返回 ErrNotFound。
func (d *AlumniDAO) GetByID(ctx context.Context, id uint64) (*model.Alumni, error) {
	var row model.Alumni
	err := d.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Create 插入一条记录，回填自增 ID。
func (d *AlumniDAO) Create(ctx context.Context, row *model.Alumni) error {
	return d.db.WithContext(ctx).Create(row).Error
}

// Count 统计有效记录数，供健康检查与统计接口使用。
func (d *AlumniDAO) Count(ctx context.Context) (int64, error) {
	var n int64
	err := d.db.WithContext(ctx).Model(&model.Alumni{}).Where("deleted_at IS NULL").Count(&n).Error
	return n, err
}
