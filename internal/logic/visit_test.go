package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	"YuLing_senior_high_school/internal/dao"
)

func validPageview() PageviewRequest {
	return PageviewRequest{EventKey: "key-one", VisitorID: "browser-a", Path: "/directory", Source: "direct"}
}

func TestRecordPageviewValidation(t *testing.T) {
	ctx := context.Background()
	l := NewVisitLogic(dao.NewMemoryStore())
	cases := []PageviewRequest{
		{EventKey: "", VisitorID: "browser-a", Path: "/"},
		{EventKey: "k", VisitorID: "bad id!", Path: "/"},
		{EventKey: "k", VisitorID: "browser-a", Path: "directory"},
		{EventKey: "k", VisitorID: "browser-a", Path: "/directory?phone=138"},
		{EventKey: "k", VisitorID: "browser-a", Path: "/" + strings.Repeat("a", 130)},
		{EventKey: "k", VisitorID: "browser-a", Path: "/", Source: "Bad Source"},
	}
	for i, req := range cases {
		if err := l.RecordPageview(ctx, req, "1.2.3.4", "ua"); err == nil {
			t.Fatalf("case %d 应被校验拦截: %+v", i, req)
		}
	}
}

func TestPageviewIdempotentAndOverview(t *testing.T) {
	ctx := context.Background()
	store := dao.NewMemoryStore()
	l := NewVisitLogic(store)
	if err := l.RecordPageview(ctx, validPageview(), "1.2.3.4", "Mozilla Mobile"); err != nil {
		t.Fatal(err)
	}
	// 同 event_key 重复上报（网络重试）不产生第二条记录
	if err := l.RecordPageview(ctx, validPageview(), "1.2.3.4", "Mozilla Mobile"); err != nil {
		t.Fatal(err)
	}
	second := validPageview()
	second.EventKey, second.Path, second.Source = "key-two", "/", "m.qq.com"
	if err := l.RecordPageview(ctx, second, "5.6.7.8", "Mozilla"); err != nil {
		t.Fatal(err)
	}
	third := validPageview()
	third.EventKey, third.VisitorID = "key-three", "browser-b"
	if err := l.RecordPageview(ctx, third, "9.9.9.9", "Mozilla"); err != nil {
		t.Fatal(err)
	}
	out, err := l.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out.Today.PV != 3 || out.Today.UV != 2 || out.Today.IPs != 3 {
		t.Fatalf("今日口径错误: %+v", out.Today)
	}
	if out.Total.PV != 3 || out.Total.UV != 2 {
		t.Fatalf("累计口径错误: %+v", out.Total)
	}
	found := false
	for _, d := range out.Days {
		if d.Day == time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02") {
			found = true
			if d.PV != 3 || d.UV != 2 {
				t.Fatalf("趋势聚合错误: %+v", d)
			}
		}
	}
	if !found {
		t.Fatal("趋势中缺少今天")
	}
	if len(out.Pages) != 2 || out.Pages[0].Path != "/directory" || out.Pages[0].PV != 2 {
		t.Fatalf("Top 页面排序错误: %+v", out.Pages)
	}
	if len(out.Recent) != 3 || out.Recent[0].EventKey != "" || out.Recent[0].IPHash != "" || len(out.Recent[0].VisitorHash) != 12 {
		t.Fatalf("明细未正确脱敏: %+v", out.Recent[0])
	}
	if out.Recent[0].Device != "desktop" || out.Recent[2].Device != "mobile" {
		t.Fatalf("设备分类错误: %+v", out.Recent)
	}
}
