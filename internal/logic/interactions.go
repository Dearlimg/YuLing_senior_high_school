package logic

import (
	"context"
	"fmt"

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
	return l.contentLikes(ctx, "post", id, liked)
}

func (l *CommunityLogic) PhotoLikes(ctx context.Context, id uint64, liked *bool) (dao.LikeState, error) {
	return l.contentLikes(ctx, "photo", id, liked)
}

func (l *CommunityLogic) contentLikes(ctx context.Context, kind string, id uint64, liked *bool) (dao.LikeState, error) {
	actor, err := CurrentActor(ctx)
	if err != nil {
		return dao.LikeState{}, err
	}
	if err := l.findContent(ctx, kind, id); err != nil {
		return dao.LikeState{}, businessError(err)
	}
	target := model.ContentLike{Kind: kind, TargetID: id, UserID: actor.UserID}
	if liked == nil {
		return l.store.Likes(ctx, target)
	}
	state, err := l.store.SetLike(ctx, target, *liked)
	return state, businessError(err)
}

func (l *CommunityLogic) findContent(ctx context.Context, kind string, id uint64) error {
	if kind == "photo" {
		_, err := l.store.FindPhoto(ctx, id)
		return err
	}
	_, err := l.store.FindPost(ctx, id)
	return err
}

func (l *CommunityLogic) Photo(ctx context.Context, id uint64) (*PhotoVO, error) {
	row, err := l.store.FindPhoto(ctx, id)
	if err != nil {
		return nil, businessError(err)
	}
	_, count, err := l.store.ListPhotoReplies(ctx, id, dao.ContentQuery{Page: 1, PageSize: 1})
	if err != nil {
		return nil, businessError(err)
	}
	state, err := l.store.Likes(ctx, model.ContentLike{Kind: "photo", TargetID: id})
	if err != nil {
		return nil, err
	}
	return &PhotoVO{ID: row.ID, Caption: row.Caption, Uploader: row.Uploader, Category: row.Category,
		CreatedAt: row.CreatedAt, ImageURL: fmt.Sprintf("/api/v1/photos/%d/image", id),
		LikeCount: state.Count, ReplyCount: count,
	}, nil
}
