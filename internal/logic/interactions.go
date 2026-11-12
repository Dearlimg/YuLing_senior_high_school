package logic

import (
	"context"

	"YuLing_senior_high_school/internal/dao"
	"YuLing_senior_high_school/internal/model"
)

func (l *CommunityLogic) Post(ctx context.Context, id uint64) (*PostVO, error) {
	row, err := l.store.FindPost(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	_, count, err := l.store.ListReplies(ctx, id, dao.ContentQuery{Page: 1, PageSize: 1})
	if err != nil {
		return nil, businessError(err)
	}
	state, err := l.store.Likes(ctx, model.ContentLike{Kind: "post", TargetID: id})
	if err != nil {
		return nil, err
	}
	row.HasImage, row.ReplyCount, row.LikeCount = len(row.Image) > 0, count, state.Count
	return &postResult([]model.Post{*row}, 1).Items[0], nil
}

func (l *CommunityLogic) PostLikes(ctx context.Context, id uint64, liked *bool) (dao.LikeState, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return dao.LikeState{}, err
	}
	if _, err := l.store.FindPost(ctx, id); err != nil {
		return dao.LikeState{}, businessError(err)
	}
	target := model.ContentLike{Kind: "post", TargetID: id, UserID: actor.UserID}
	if liked == nil {
		return l.store.Likes(ctx, target)
	}
	state, err := l.store.SetLike(ctx, target, *liked)
	return state, businessError(err)
}
