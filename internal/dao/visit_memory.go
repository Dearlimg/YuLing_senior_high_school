package dao

import (
	"context"
	"sort"
	"time"

	"YuLing_senior_high_school/internal/model"
)

// 内存实现与 SQL 语义保持一致：event_key 幂等，按东八区自然日聚合。

func (s *MemoryStore) RecordVisit(_ context.Context, event *model.VisitEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.visits {
		if row.EventKey == event.EventKey {
			return nil
		}
	}
	event.ID = uint64(len(s.visits) + 1)
	s.visits = append(s.visits, *event)
	return nil
}

func visitWindowOf(rows []model.VisitEvent, since, until *time.Time) VisitWindow {
	visitors, ips := map[string]bool{}, map[string]bool{}
	var w VisitWindow
	for _, row := range rows {
		if since != nil && row.CreatedAt.Before(*since) {
			continue
		}
		if until != nil && !row.CreatedAt.Before(*until) {
			continue
		}
		w.PV++
		visitors[row.VisitorHash] = true
		ips[row.IPHash] = true
	}
	w.UV, w.IPs = int64(len(visitors)), int64(len(ips))
	return w
}

func (s *MemoryStore) VisitOverview(_ context.Context, now time.Time) (*VisitOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := cstDayStart(now)
	yesterday := start.Add(-24 * time.Hour)
	out := &VisitOverview{Days: []VisitDayCount{}, Pages: []VisitPathCount{}, Recent: []model.VisitEvent{}}
	out.Today = visitWindowOf(s.visits, &start, nil)
	out.Yesterday = visitWindowOf(s.visits, &yesterday, &start)
	out.Total = visitWindowOf(s.visits, nil, nil)

	cst := time.FixedZone("CST", 8*3600)
	trendSince := start.Add(-13 * 24 * time.Hour)
	byDay := map[string][]model.VisitEvent{}
	pagesSince := start.Add(-6 * 24 * time.Hour)
	byPath := map[string][]model.VisitEvent{}
	for _, row := range s.visits {
		if !row.CreatedAt.Before(trendSince) {
			day := row.CreatedAt.In(cst).Format("2006-01-02")
			byDay[day] = append(byDay[day], row)
		}
		if !row.CreatedAt.Before(pagesSince) {
			byPath[row.Path] = append(byPath[row.Path], row)
		}
	}
	for day, rows := range byDay {
		w := visitWindowOf(rows, nil, nil)
		out.Days = append(out.Days, VisitDayCount{Day: day, PV: w.PV, UV: w.UV})
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Day < out.Days[j].Day })
	for path, rows := range byPath {
		w := visitWindowOf(rows, nil, nil)
		out.Pages = append(out.Pages, VisitPathCount{Path: path, PV: w.PV, UV: w.UV})
	}
	sort.Slice(out.Pages, func(i, j int) bool {
		if out.Pages[i].PV != out.Pages[j].PV {
			return out.Pages[i].PV > out.Pages[j].PV
		}
		return out.Pages[i].Path < out.Pages[j].Path
	})
	if len(out.Pages) > 10 {
		out.Pages = out.Pages[:10]
	}
	for i := len(s.visits) - 1; i >= 0 && len(out.Recent) < 30; i-- {
		out.Recent = append(out.Recent, s.visits[i])
	}
	return out, nil
}
