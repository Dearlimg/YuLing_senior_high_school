// Package logic implements business rules without depending on HTTP or an ORM.
package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/width"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

var (
	ErrInvalidParam = errors.New("参数不合法")
	ErrNotFound     = errors.New("资源不存在")
	ErrForbidden    = errors.New("编辑码或删除码不正确")
	ErrDuplicate    = errors.New("这份内容已提交，请勿重复操作")
)

var phonePattern = regexp.MustCompile(`^(1[3-9]\d{9}|\+[1-9]\d{6,14})$`)

type AlumniLogic struct{ store dao.Store }

func NewAlumniLogic(store dao.Store) *AlumniLogic { return &AlumniLogic{store: store} }

type ListRequest struct {
	GraduationYear int    `form:"graduation_year"`
	City           string `form:"city"`
	Keyword        string `form:"keyword"`
	University     string `form:"university"`
	ClassName      string `form:"class_name"`
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
}

// AlumniVO is a deliberate public allowlist. Contact fields never enter this type.
type AlumniVO struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Identity        string `json:"identity"`
	GraduationYear  int    `json:"graduation_year"`
	ClassName       string `json:"class_name"`
	City            string `json:"city"`
	University      string `json:"university"`
	DreamUniversity string `json:"dream_university"`
	Major           string `json:"major"`
	Education       string `json:"education"`
	Company         string `json:"company"`
	Position        string `json:"position"`
	Message         string `json:"message"`
	AvatarColor     string `json:"avatar_color"`
	ContactPublic   bool   `json:"contact_public"`
}

type OwnerVO struct {
	AlumniVO
	Phone  string `json:"phone"`
	Wechat string `json:"wechat"`
	Email  string `json:"email"`
}

type ListResult struct {
	Items    []AlumniVO `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

type ClassResult struct {
	Items []dao.ClassGroup `json:"items"`
	Total int64            `json:"total"`
}

func (l *AlumniLogic) List(ctx context.Context, req ListRequest) (*ListResult, error) {
	q, err := listQuery(req)
	if err != nil {
		return nil, err
	}
	rows, total, err := l.store.ListAlumni(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]AlumniVO, 0, len(rows))
	for _, r := range rows {
		items = append(items, toVO(r))
	}
	return &ListResult{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (l *AlumniLogic) GetByID(ctx context.Context, id uint64) (*AlumniVO, error) {
	row, err := l.store.FindAlumni(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	vo := toVO(*row)
	return &vo, nil
}

type CreateRequest struct {
	Name            string `json:"name"`
	Identity        string `json:"identity"`
	GraduationYear  int    `json:"graduation_year"`
	ClassName       string `json:"class_name"`
	City            string `json:"city"`
	University      string `json:"university"`
	DreamUniversity string `json:"dream_university"`
	Major           string `json:"major"`
	Education       string `json:"education"`
	Company         string `json:"company"`
	Position        string `json:"position"`
	Phone           string `json:"phone"`
	Wechat          string `json:"wechat"`
	Email           string `json:"email"`
	Message         string `json:"message"`
	AvatarColor     string `json:"avatar_color"`
	ContactPublic   bool   `json:"contact_public"`
	EditCode        string `json:"edit_code"`
}

type CreateResult struct {
	ID       uint64 `json:"id"`
	EditCode string `json:"edit_code,omitempty"`
}

func (l *AlumniLogic) Create(ctx context.Context, req CreateRequest, key string) (*CreateResult, error) {
	row, err := validateAlumni(req)
	if err != nil {
		return nil, err
	}
	code, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	row.EditCodeHash = hash
	row.CreateKey = createKey(key, req)
	if err := l.store.CreateAlumni(ctx, row); err != nil {
		return nil, businessError(err)
	}
	return &CreateResult{ID: row.ID, EditCode: code}, nil
}

func (l *AlumniLogic) Unlock(ctx context.Context, id uint64, code string) (*OwnerVO, error) {
	row, err := l.store.FindAlumni(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	if !matchesSecret(code, row.EditCodeHash) {
		return nil, ErrForbidden
	}
	return &OwnerVO{AlumniVO: toVO(*row), Phone: row.Phone, Wechat: row.Wechat, Email: row.Email}, nil
}

func (l *AlumniLogic) Update(ctx context.Context, id uint64, req CreateRequest) (*CreateResult, error) {
	old, err := l.store.FindAlumni(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	if !matchesSecret(req.EditCode, old.EditCodeHash) {
		return nil, ErrForbidden
	}
	row, err := validateAlumni(req)
	if err != nil {
		return nil, err
	}
	row.ID = id
	row.EditCodeHash = old.EditCodeHash
	row.CreatedAt = old.CreatedAt
	row.CreateKey = old.CreateKey
	if err := l.store.UpdateAlumni(ctx, row); err != nil {
		return nil, businessError(err)
	}
	return &CreateResult{ID: id}, nil
}

func (l *AlumniLogic) Meta(ctx context.Context) (dao.DirectoryMeta, error) {
	return l.store.DirectoryMeta(ctx)
}
func (l *AlumniLogic) Classes(ctx context.Context, req ListRequest) (*ClassResult, error) {
	q, err := listQuery(req)
	if err != nil {
		return nil, err
	}
	rows, total, err := l.store.Classes(ctx, q)
	return &ClassResult{Items: rows, Total: total}, err
}
func (l *AlumniLogic) Stats(ctx context.Context) (dao.Stats, error) { return l.store.Stats(ctx) }
func (l *AlumniLogic) Universities(ctx context.Context) ([]dao.University, int64, error) {
	rows, err := l.store.Universities(ctx)
	var total int64
	for _, r := range rows {
		total += r.Count
	}
	return rows, total, err
}

func listQuery(req ListRequest) (dao.ListQuery, error) {
	if utf8.RuneCountInString(req.Keyword) > 100 {
		return dao.ListQuery{}, invalid("搜索关键词不能超过 100 字")
	}
	page, size := pagination(req.Page, req.PageSize)
	return dao.ListQuery{
		GraduationYear: req.GraduationYear,
		City:           strings.TrimSpace(req.City),
		Keyword:        strings.TrimSpace(req.Keyword),
		University:     NormalizeUniversity(req.University),
		ClassName:      strings.TrimSpace(req.ClassName),
		Page:           page,
		PageSize:       size,
	}, nil
}

func validateAlumni(req CreateRequest) (*model.Alumni, error) {
	row := &model.Alumni{
		Name:            strings.TrimSpace(req.Name),
		Identity:        req.Identity,
		GraduationYear:  req.GraduationYear,
		ClassName:       strings.TrimSpace(req.ClassName),
		City:            strings.TrimSpace(req.City),
		University:      NormalizeUniversity(req.University),
		DreamUniversity: NormalizeUniversity(req.DreamUniversity),
		Major:           strings.TrimSpace(req.Major),
		Education:       strings.TrimSpace(req.Education),
		Company:         strings.TrimSpace(req.Company),
		Position:        strings.TrimSpace(req.Position),
		Phone:           strings.TrimSpace(req.Phone),
		Wechat:          strings.TrimSpace(req.Wechat),
		Email:           strings.TrimSpace(req.Email),
		Message:         strings.TrimSpace(req.Message),
		AvatarColor:     req.AvatarColor,
		ContactPublic:   req.ContactPublic,
		Status:          "active",
	}
	if row.Identity == "" {
		row.Identity = "alumni"
	}
	if row.AvatarColor == "" {
		row.AvatarColor = "red"
	}
	if row.Identity != "alumni" && row.Identity != "student" {
		return nil, invalid("登记身份不正确")
	}
	currentYear := time.Now().Year()
	maxYear := currentYear
	if row.Identity == "student" {
		maxYear += 6
	}
	if row.GraduationYear < 1903 || row.GraduationYear > maxYear {
		return nil, invalid(fmt.Sprintf("毕业年份需在 1903～%d 之间", maxYear))
	}
	if row.Identity == "student" && row.GraduationYear < currentYear {
		return nil, invalid("在读同学的预计毕业年份不能早于今年")
	}
	checks := []struct {
		label, value string
		max          int
		required     bool
	}{
		{label: "姓名", value: row.Name, max: 30, required: true}, {label: "班级", value: row.ClassName, max: 30, required: row.Identity == "student"},
		{label: "城市", value: row.City, max: 50}, {label: "大学", value: row.University, max: 100}, {label: "理想大学", value: row.DreamUniversity, max: 100},
		{label: "专业", value: row.Major, max: 50}, {label: "单位", value: row.Company, max: 100}, {label: "职位", value: row.Position, max: 50},
		{label: "微信", value: row.Wechat, max: 50}, {label: "邮箱", value: row.Email, max: 100}, {label: "寄语", value: row.Message, max: 120},
	}
	for _, c := range checks {
		if err := validateText(c.label, c.value, c.max, c.required); err != nil {
			return nil, err
		}
	}
	if row.Phone != "" && !phonePattern.MatchString(row.Phone) {
		return nil, invalid("手机号格式不正确，请使用大陆手机号或 + 国际区号格式")
	}
	if row.Email != "" {
		address, err := mail.ParseAddress(row.Email)
		if err != nil || address.Address != row.Email {
			return nil, invalid("邮箱格式不正确")
		}
	}
	if !slicesContain([]string{"red", "blue", "gold", "pink", "green"}, row.AvatarColor) {
		return nil, invalid("请选择有效的头像颜色")
	}
	if !slicesContain([]string{"", "高中", "大专", "本科", "硕士", "博士", "其他"}, row.Education) {
		return nil, invalid("学历选项不正确")
	}
	if row.Identity == "student" {
		row.University = ""
	} else {
		row.DreamUniversity = ""
	}
	return row, nil
}

func toVO(m model.Alumni) AlumniVO {
	return AlumniVO{
		ID: m.ID, Name: m.Name, Identity: m.Identity,
		GraduationYear: m.GraduationYear, ClassName: m.ClassName, City: m.City,
		University: m.University, DreamUniversity: m.DreamUniversity,
		Major: m.Major, Education: m.Education, Company: m.Company,
		Position: m.Position, Message: m.Message,
		AvatarColor: m.AvatarColor, ContactPublic: m.ContactPublic,
	}
}

// NormalizeUniversity makes aliases and whitespace equivalent before storing and aggregating.
func NormalizeUniversity(value string) string {
	value = width.Fold.String(value)
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	aliases := map[string]string{
		"西安交大": "西安交通大学", "西交大": "西安交通大学",
		"西北工大": "西北工业大学", "西工大": "西北工业大学",
		"西电": "西安电子科技大学", "北大": "北京大学", "清华": "清华大学",
		"人大": "中国人民大学", "陕师大": "陕西师范大学", "西大": "西北大学",
		"北师大": "北京师范大学", "哈工大": "哈尔滨工业大学", "上交": "上海交通大学",
	}
	if canonical, ok := aliases[value]; ok {
		return canonical
	}
	return value
}

func newSecret() (string, string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	code := base64.RawURLEncoding.EncodeToString(bytes)
	return code, secretHash(code), nil
}
func secretHash(code string) string {
	hash := sha256.Sum256([]byte(code))
	return hex.EncodeToString(hash[:])
}
func matchesSecret(code, hash string) bool {
	if code == "" || hash == "" || len(code) > 80 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(secretHash(code)), []byte(hash)) == 1
}
func createKey(key string, payload any) *string {
	if key == "" {
		encoded, _ := json.Marshal(payload)
		key = string(encoded)
	}
	hash := secretHash(key)
	return &hash
}
func pagination(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalidParam, message) }
func validateText(label, value string, max int, required bool) error {
	if required && value == "" {
		return invalid(label + "不能为空")
	}
	if utf8.RuneCountInString(value) > max {
		return invalid(fmt.Sprintf("%s不能超过 %d 字", label, max))
	}
	return nil
}
func slicesContain(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func businessError(err error) error {
	if errors.Is(err, dao.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, dao.ErrDuplicate) {
		return ErrDuplicate
	}
	return err
}
