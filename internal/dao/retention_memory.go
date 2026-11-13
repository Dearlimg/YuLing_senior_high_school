package dao

import (
	"context"
	"fmt"
	"slices"
	"time"

	"YuLing_senior_high_school/internal/model"
)

func (s *MemoryStore) SaveSubscription(_ context.Context, row *model.ClassSubscription, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	for _, old := range s.subscriptions {
		if old.UserID != row.UserID {
			continue
		}
		if old.Active {
			count++
		}
		if old.GraduationYear == row.GraduationYear && old.ClassName == row.ClassName {
			if old.Active == active {
				return nil
			}
			row.ID, row.CreatedAt = old.ID, old.CreatedAt
			if old.Active {
				count--
			}
		}
	}
	if active && count >= 20 {
		return ErrSubscriptionLimit
	}
	if row.ID == 0 && !active {
		return nil
	}
	if row.ID == 0 {
		row.ID = uint64(len(s.subscriptions) + 1)
		row.CreatedAt = time.Now()
	}
	row.Active, row.UpdatedAt = active, time.Now()
	if active {
		row.SinceAt, row.ReadAt = time.Now().UTC(), time.Now().UTC()
	}
	s.subscriptions[row.ID] = *row
	return nil
}

func (s *MemoryStore) Subscriptions(_ context.Context, userID uint64) ([]model.ClassSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []model.ClassSubscription{}
	for _, row := range s.subscriptions {
		if row.UserID == userID && row.Active {
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b model.ClassSubscription) int { return compareID(b.ID, a.ID) })
	return rows, nil
}

func (s *MemoryStore) ReadActivities(_ context.Context, userID uint64, through time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, row := range s.subscriptions {
		if row.UserID == userID && row.Active && row.ReadAt.Before(through) {
			row.ReadAt = through
			s.subscriptions[id] = row
		}
	}
	return nil
}

func (s *MemoryStore) ActivityFeed(_ context.Context, q FeedQuery) (FeedResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := FeedResult{Items: []Activity{}, AsOf: q.AsOf}
	appendActivity := func(item Activity) {
		if item.OccurredAt.After(q.AsOf) {
			return
		}
		for _, sub := range s.subscriptions {
			matches := sub.UserID == q.UserID && sub.Active && sub.GraduationYear == item.GraduationYear
			if !matches || sub.ClassName != item.ClassName || item.OccurredAt.Before(sub.SinceAt) {
				continue
			}
			item.Unread = item.OccurredAt.After(sub.ReadAt)
			if item.Unread {
				result.Unread++
			}
			result.Items = append(result.Items, item)
			break
		}
	}
	for _, card := range s.matching(ListQuery{}) {
		appendActivity(Activity{Kind: "alumni", TargetID: card.ID, Name: card.Name, Summary: card.Message,
			GraduationYear: card.GraduationYear, ClassName: card.ClassName, OccurredAt: card.UpdatedAt})
	}
	cardFor := func(userID uint64) (model.Alumni, bool) {
		user, ok := s.users[userID]
		if !ok {
			return model.Alumni{}, false
		}
		card, ok := s.alumni[user.AlumniID]
		return card, ok && card.Status == "active" && card.DeletedAt == nil
	}
	for _, post := range s.posts {
		card, ok := cardFor(post.UserID)
		if !ok || post.Status != "active" {
			continue
		}
		summary := []rune(post.Content)
		if len(summary) > 80 {
			summary = summary[:80]
		}
		appendActivity(Activity{Kind: "post", TargetID: post.ID, Name: card.Name, Summary: string(summary),
			GraduationYear: card.GraduationYear, ClassName: card.ClassName, OccurredAt: post.CreatedAt})
	}
	for _, photo := range s.photos {
		card, ok := cardFor(photo.UserID)
		if !ok || photo.Status != "active" {
			continue
		}
		appendActivity(Activity{Kind: "photo", TargetID: photo.ID, Name: card.Name, Summary: photo.Caption,
			GraduationYear: card.GraduationYear, ClassName: card.ClassName, OccurredAt: photo.CreatedAt})
	}
	slices.SortFunc(result.Items, func(a, b Activity) int {
		if cmp := b.OccurredAt.Compare(a.OccurredAt); cmp != 0 {
			return cmp
		}
		if a.Kind < b.Kind {
			return -1
		}
		if a.Kind > b.Kind {
			return 1
		}
		return compareID(b.TargetID, a.TargetID)
	})
	result.Total = int64(len(result.Items))
	result.Items = pageSlice(result.Items, q.Page, q.PageSize)
	return result, nil
}

func (s *MemoryStore) AnnualReport(_ context.Context, w ReportWindow) (AnnualReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := AnnualReport{Universities: []University{}, TopClasses: []ClassGroup{}}
	cities := map[string]bool{}
	classes := map[string]ClassGroup{}
	universities := map[string]int64{}
	within := func(at time.Time) bool { return !at.Before(w.Since) && at.Before(w.Until) }
	for _, card := range s.matching(ListQuery{}) {
		if !within(card.CreatedAt) {
			continue
		}
		result.Registrations++
		if card.Identity == "alumni" {
			result.Graduates++
			if card.University != "" {
				universities[card.University]++
			}
		}
		if card.Identity == "student" {
			result.Students++
		}
		if card.City != "" {
			cities[card.City] = true
		}
		if card.ClassName != "" {
			key := fmt.Sprintf("%d:%s", card.GraduationYear, card.ClassName)
			group := classes[key]
			group.ClassName = card.ClassName
			group.GraduationYear = card.GraduationYear
			group.Count++
			classes[key] = group
		}
	}
	for _, photo := range s.photos {
		if photo.Status == "active" && within(photo.CreatedAt) {
			result.Photos++
		}
	}
	for _, post := range s.posts {
		if post.Status == "active" && within(post.CreatedAt) {
			result.Posts++
		}
	}
	result.Cities, result.Classes = int64(len(cities)), int64(len(classes))
	result.Universities = sortedUniversities(universities)
	for _, group := range classes {
		result.TopClasses = append(result.TopClasses, group)
	}
	slices.SortFunc(result.TopClasses, func(a, b ClassGroup) int {
		if a.Count != b.Count {
			if a.Count > b.Count {
				return -1
			}
			return 1
		}
		if a.GraduationYear != b.GraduationYear {
			return b.GraduationYear - a.GraduationYear
		}
		if a.ClassName < b.ClassName {
			return -1
		}
		if a.ClassName > b.ClassName {
			return 1
		}
		return 0
	})
	if len(result.TopClasses) > 10 {
		result.TopClasses = result.TopClasses[:10]
	}
	return result, nil
}
