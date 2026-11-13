package logic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

var ErrUnauthorized = errors.New("请先登录，或登录已过期")

const SessionTTL = 7 * 24 * time.Hour

var accountPattern = regexp.MustCompile(`^[a-z0-9_]{4,32}$`)

// A fixed valid bcrypt hash keeps nonexistent-account checks comparable to wrong passwords.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

type AuthLogic struct {
	store    dao.Store
	sessions dao.SessionStore
}

func NewAuthLogic(store dao.Store, sessions dao.SessionStore) *AuthLogic {
	return &AuthLogic{store: store, sessions: sessions}
}

type RegisterRequest struct {
	CreateRequest
	Account  string `json:"account"`
	Password string `json:"password"`
	AlumniID uint64 `json:"alumni_id"`
}

type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type AuthResult struct {
	UserID    uint64    `json:"-"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	ID        uint64    `json:"id"`
}

type MeResult struct {
	UserID  uint64  `json:"user_id"`
	IsAdmin bool    `json:"is_admin"`
	Account string  `json:"account"`
	Profile OwnerVO `json:"profile"`
}

// Actor is populated from the Redis session and current database card, never request fields.
type Actor struct {
	IsAdmin bool
	UserID  uint64
	Account string
	Card    model.Alumni
}

type actorKey struct{}

func WithActor(ctx context.Context, actor *Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func CurrentActor(ctx context.Context) (*Actor, error) {
	actor, ok := ctx.Value(actorKey{}).(*Actor)
	if !ok || actor == nil || actor.UserID == 0 {
		return nil, ErrUnauthorized
	}
	return actor, nil
}

func (l *AuthLogic) Register(ctx context.Context, req RegisterRequest, key string) (*AuthResult, error) {
	account := strings.ToLower(strings.TrimSpace(req.Account))
	if !accountPattern.MatchString(account) {
		return nil, invalid("账号须为 4～32 位字母、数字或下划线")
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		return nil, invalid("密码须为 8～72 字节，请使用较长的独立密码")
	}
	var card *model.Alumni
	var err error
	if req.AlumniID != 0 {
		card, err = l.store.FindAlumni(ctx, req.AlumniID)
		if err != nil {
			return nil, businessError(err)
		}
		if !matchesSecret(req.EditCode, card.EditCodeHash) {
			return nil, ErrForbidden
		}
	} else {
		card, err = validateAlumni(req.CreateRequest)
		if err != nil {
			return nil, err
		}
		card.CreateKey = createKey(account+":"+key, nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{Account: account, PasswordHash: string(hash)}
	if err := l.store.RegisterUser(ctx, user, card); err != nil {
		if errors.Is(err, dao.ErrDuplicate) {
			return nil, fmtDuplicateAccount()
		}
		return nil, businessError(err)
	}
	return l.issue(ctx, user)
}

func fmtDuplicateAccount() error {
	return errors.Join(ErrDuplicate, errors.New("账号已存在，或此校友卡已绑定账号，请登录"))
}

func (l *AuthLogic) Login(ctx context.Context, req LoginRequest) (*AuthResult, error) {
	account := strings.ToLower(strings.TrimSpace(req.Account))
	if !accountPattern.MatchString(account) || len(req.Password) > 72 {
		return nil, ErrUnauthorized
	}
	user, err := l.store.FindUser(ctx, account)
	if errors.Is(err, dao.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, ErrUnauthorized
	}
	if _, err := l.store.FindAlumni(ctx, user.AlumniID); err != nil {
		return nil, businessError(err)
	}
	return l.issue(ctx, user)
}

func (l *AuthLogic) issue(ctx context.Context, user *model.User) (*AuthResult, error) {
	token, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	if err := l.sessions.SaveSession(ctx, hash, user.ID, SessionTTL); err != nil {
		return nil, err
	}
	return &AuthResult{UserID: user.ID, Token: token, ExpiresAt: time.Now().Add(SessionTTL), ID: user.AlumniID}, nil
}

func (l *AuthLogic) Authenticate(ctx context.Context, token string) (*Actor, error) {
	if len(token) != 32 {
		return nil, ErrUnauthorized
	}
	id, err := l.sessions.SessionUser(ctx, secretHash(token))
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	user, err := l.store.FindUserByID(ctx, id)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	card, err := l.store.FindAlumni(ctx, user.AlumniID)
	if errors.Is(err, dao.ErrNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return &Actor{IsAdmin: user.IsAdmin, UserID: user.ID, Account: user.Account, Card: *card}, nil
}

func (l *AuthLogic) Logout(ctx context.Context, token string) error {
	return l.sessions.DeleteSession(ctx, secretHash(token))
}

func (l *AuthLogic) Me(ctx context.Context) (*MeResult, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return nil, err
	}
	return &MeResult{UserID: actor.UserID, IsAdmin: actor.IsAdmin, Account: actor.Account, Profile: ownerVO(actor.Card)}, nil
}

func ownerVO(card model.Alumni) OwnerVO {
	return OwnerVO{AlumniVO: toVO(card), Phone: card.Phone, Wechat: card.Wechat, Email: card.Email,
		PhonePublic: card.PhonePublic, WechatPublic: card.WechatPublic}
}

func (l *AuthLogic) UpdateCard(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return nil, err
	}
	card, err := validateAlumni(req)
	if err != nil {
		return nil, err
	}
	card.ID = actor.Card.ID
	card.EditCodeHash = actor.Card.EditCodeHash
	if err := l.store.UpdateAlumni(ctx, card); err != nil {
		return nil, businessError(err)
	}
	return &CreateResult{ID: card.ID}, nil
}

func actorCreateKey(userID uint64, key string, payload any) *string {
	if key != "" {
		return createKey(fmt.Sprintf("%d:%s", userID, key), nil)
	}
	return createKey("", struct {
		UserID  uint64
		Payload any
	}{UserID: userID, Payload: payload})
}
