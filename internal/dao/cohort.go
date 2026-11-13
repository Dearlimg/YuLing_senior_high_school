package dao

import (
	"context"
	"slices"
)

func (s *SQLStore) CohortUniversities(ctx context.Context, year int) ([]University, error) {
	rows := []University{}
	err := s.alumniQuery(ctx, ListQuery{GraduationYear: year}).Where("identity = ? AND university <> ''", "alumni").
		Select("university AS name, COUNT(*) AS count").Group("university").Order("count DESC, university ASC").Scan(&rows).Error
	return rows, err
}

func (s *MemoryStore) CohortUniversities(_ context.Context, year int) ([]University, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int64{}
	for _, row := range s.matching(ListQuery{GraduationYear: year}) {
		if row.Identity == "alumni" && row.University != "" {
			counts[row.University]++
		}
	}
	return sortedUniversities(counts), nil
}

func sortedUniversities(counts map[string]int64) []University {
	rows := []University{}
	for name, count := range counts {
		rows = append(rows, University{Name: name, Count: count})
	}
	slices.SortFunc(rows, func(a, b University) int {
		if a.Count != b.Count {
			if a.Count > b.Count {
				return -1
			}
			return 1
		}
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return rows
}
