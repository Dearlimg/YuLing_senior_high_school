package logic

import (
	"context"
	"errors"
	"testing"

	"YuLing_senior_high_school/internal/dao"
)

func TestPostLikesAreIdempotentAndRespectVisibility(t *testing.T) {
	ctx := testActorContext()
	store := dao.NewMemoryStore()
	l := NewCommunityLogic(store)
	id, _ := l.CreatePost(ctx, PostRequest{Content: "同窗近况"})
	liked := true
	if _, err := l.PostLikes(context.Background(), id, &liked); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("anonymous like: %v", err)
	}
	for range 2 {
		state, err := l.PostLikes(ctx, id, &liked)
		if err != nil || !state.Liked || state.Count != 1 {
			t.Fatalf("like: %+v %v", state, err)
		}
	}
	post, err := l.Post(ctx, id)
	if err != nil || post.LikeCount != 1 {
		t.Fatalf("detail: %+v %v", post, err)
	}
	liked = false
	state, err := l.PostLikes(ctx, id, &liked)
	if err != nil || state.Liked || state.Count != 0 {
		t.Fatalf("unlike: %+v %v", state, err)
	}
	_ = l.Moderate(ctx, "post", id, ModerationRequest{Action: "hidden", Reason: "test"})
	if _, err := l.Post(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden detail: %v", err)
	}
	if _, err := l.PostLikes(ctx, id, &liked); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden like: %v", err)
	}
}

func TestRepliesCanTargetAnyVisibleCommentInSamePost(t *testing.T) {
	ctx := testActorContext()
	l := NewCommunityLogic(dao.NewMemoryStore())
	id, _ := l.CreatePost(ctx, PostRequest{Content: "讨论"})
	_, _ = l.Reply(ctx, id, ReplyRequest{Content: "第一层"}, "r1")
	_, _ = l.Reply(ctx, id, ReplyRequest{Content: "第二层", ParentID: 1}, "r2")
	_, err := l.Reply(ctx, id, ReplyRequest{Content: "回复第二层", ParentID: 2}, "r3")
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := l.Replies(ctx, id, ContentRequest{})
	if rows.Total != 3 || rows.Items[2].ParentID != 2 || rows.Items[2].ParentNickname != "测试校友" {
		t.Fatalf("reply target lost: %+v", rows)
	}
	other, _ := l.CreatePost(ctx, PostRequest{Content: "另一个帖子"})
	if _, err := l.Reply(ctx, other, ReplyRequest{Content: "跨帖回复", ParentID: 2}, "cross"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-post target: %v", err)
	}
	_ = l.Moderate(ctx, "reply", 2, ModerationRequest{Action: "hidden", Reason: "test"})
	if _, err := l.Reply(ctx, id, ReplyRequest{Content: "隐藏目标", ParentID: 2}, "hidden"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden target: %v", err)
	}
	rows, _ = l.Replies(ctx, id, ContentRequest{})
	if rows.Items[1].ParentNickname != "" {
		t.Fatal("hidden target nickname leaked")
	}
}
