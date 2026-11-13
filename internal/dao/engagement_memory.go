package dao

import (
	"context"
	"slices"
	"time"

	"YuLing_senior_high_school/internal/model"
)

func (s *MemoryStore) ApplyOrganizer(_ context.Context, row *model.ClassOrganizer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	card, ok := s.alumni[row.AlumniID]
	if !ok || card.Status != "active" || card.ClassName == "" {
		return ErrNotFound
	}
	row.GraduationYear, row.ClassName = card.GraduationYear, card.ClassName
	row.CreatedAt, row.UpdatedAt = time.Now(), time.Now()
	for _, old := range s.organizers {
		if old.UserID == row.UserID {
			row.ID, row.CreatedAt = old.ID, old.CreatedAt
			break
		}
	}
	if row.ID == 0 {
		row.ID = uint64(len(s.organizers) + 1)
	}
	s.organizers[row.ID] = *row
	return nil
}

func (s *MemoryStore) ListOrganizers(_ context.Context, q OrganizerQuery) ([]model.ClassOrganizer, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []model.ClassOrganizer{}
	for _, row := range s.organizers {
		card := s.alumni[row.AlumniID]
		public := row.Status == "active" && card.Status == "active" && card.DeletedAt == nil
		currentClass := card.GraduationYear == row.GraduationYear && card.ClassName == row.ClassName
		if !q.Private && (!public || !currentClass) {
			continue
		}
		if q.UserID != 0 && q.UserID != row.UserID {
			continue
		}
		if q.GraduationYear != 0 && q.GraduationYear != row.GraduationYear {
			continue
		}
		if q.ClassName != "" && q.ClassName != row.ClassName {
			continue
		}
		row.Name = card.Name
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b model.ClassOrganizer) int { return compareID(b.ID, a.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) appendAudit(row model.AuditLog) {
	row.ID, row.CreatedAt = uint64(len(s.audits)+1), time.Now()
	s.audits = append(s.audits, row)
}

func (s *MemoryStore) ReviewOrganizer(_ context.Context, req OrganizerReview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.organizers[req.ID]
	if !ok || !slices.Contains(organizerPrevious(req.Status), row.Status) {
		return ErrNotFound
	}
	if req.UserID != 0 && req.UserID != row.UserID {
		return ErrNotFound
	}
	row.Status, row.UpdatedAt = req.Status, time.Now()
	s.organizers[row.ID] = row
	s.appendAudit(req.Audit)
	return nil
}

func (s *MemoryStore) CreateRecovery(_ context.Context, row *model.CardRecovery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	card, ok := s.alumni[row.AlumniID]
	if !ok || card.Status != "active" || card.EditCodeHash == "" {
		return ErrNotFound
	}
	row.ID, row.CreatedAt, row.UpdatedAt = uint64(len(s.recoveries)+1), time.Now(), time.Now()
	s.recoveries[row.ID] = *row
	return nil
}

func (s *MemoryStore) FindRecovery(_ context.Context, id uint64) (*model.CardRecovery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.recoveries[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &row, nil
}

func (s *MemoryStore) ListRecoveries(_ context.Context, q ContentQuery) ([]model.CardRecovery, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []model.CardRecovery{}
	for _, row := range s.recoveries {
		if q.Status != "" && q.Status != row.Status {
			continue
		}
		row.Name, row.ReceiptHash = s.alumni[row.AlumniID].Name, ""
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b model.CardRecovery) int { return compareID(b.ID, a.ID) })
	return pageSlice(rows, q.Page, q.PageSize), int64(len(rows)), nil
}

func (s *MemoryStore) ReviewRecovery(_ context.Context, req RecoveryReview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.recoveries[req.ID]
	if !ok || row.Status != "pending" || !row.ExpiresAt.After(time.Now()) {
		return ErrNotFound
	}
	row.Status, row.UpdatedAt = req.Status, time.Now()
	s.recoveries[row.ID] = row
	s.appendAudit(req.Audit)
	return nil
}

func (s *MemoryStore) CompleteRecovery(_ context.Context, req RecoveryCompletion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	card, ok := s.alumni[req.AlumniID]
	if !ok || card.Status != "active" || card.DeletedAt != nil {
		return ErrNotFound
	}
	row, ok := s.recoveries[req.ID]
	valid := ok && row.AlumniID == req.AlumniID && row.ReceiptHash == req.ReceiptHash
	approved := row.Status == "approved" && row.ExpiresAt.After(time.Now()) && card.EditCodeHash != ""
	if !valid || !approved {
		return ErrNotFound
	}
	card.EditCodeHash = req.NewEditHash
	s.alumni[card.ID] = card
	for id, other := range s.recoveries {
		if other.AlumniID == card.ID && slices.Contains([]string{"pending", "approved"}, other.Status) {
			other.Status = "revoked"
			if id == row.ID {
				other.Status = "used"
			}
			other.UpdatedAt = time.Now()
			s.recoveries[id] = other
		}
	}
	s.appendAudit(req.Audit)
	return nil
}
