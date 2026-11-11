package dao

import (
	"context"
	"time"

	"YuLing_senior_high_school/internal/model"
)

type memorySession struct {
	userID  uint64
	expires time.Time
}

func (s *MemoryStore) FindUser(_ context.Context, account string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range s.users {
		if user.Account == account {
			return &user, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) FindUserByID(_ context.Context, id uint64) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &user, nil
}

func (s *MemoryStore) FindUserByAlumni(_ context.Context, id uint64) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range s.users {
		if user.AlumniID == id {
			return &user, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) RegisterUser(_ context.Context, user *model.User, card *model.Alumni) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if existing.Account == user.Account || (card.ID != 0 && existing.AlumniID == card.ID) {
			return ErrDuplicate
		}
	}
	if card.ID == 0 {
		if err := s.useKey("alumni", card.CreateKey); err != nil {
			return err
		}
		card.ID = uint64(len(s.alumni) + 1)
		card.CreatedAt = time.Now()
		card.UpdatedAt = card.CreatedAt
		s.alumni[card.ID] = *card
	} else if old, ok := s.alumni[card.ID]; !ok || old.Status != "active" || old.DeletedAt != nil {
		return ErrNotFound
	}
	claimed := s.alumni[card.ID]
	claimed.EditCodeHash = ""
	s.alumni[card.ID] = claimed
	user.ID = uint64(len(s.users) + 1)
	user.AlumniID = card.ID
	user.CreatedAt = time.Now()
	s.users[user.ID] = *user
	return nil
}

func (s *MemoryStore) SaveSession(_ context.Context, hash string, id uint64, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, session := range s.sessions {
		if time.Now().After(session.expires) {
			delete(s.sessions, key)
		}
	}
	s.sessions[hash] = memorySession{userID: id, expires: time.Now().Add(ttl)}
	return nil
}

func (s *MemoryStore) SessionUser(_ context.Context, hash string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[hash]
	if !ok || time.Now().After(session.expires) {
		delete(s.sessions, hash)
		return 0, ErrNotFound
	}
	return session.userID, nil
}

func (s *MemoryStore) DeleteSession(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hash)
	return nil
}
