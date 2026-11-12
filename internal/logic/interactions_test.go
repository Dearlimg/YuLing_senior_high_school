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
