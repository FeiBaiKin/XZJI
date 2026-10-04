package content

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostUpdate sitemap 用的内容更新信息。
type PostUpdate struct {
	ID        int64
	UpdatedAt time.Time
}

// PublishedUpdates 已发布内容的 ID 和更新时间，供 sitemap 使用。
func (s *Service) PublishedUpdates(ctx context.Context, at time.Time) ([]PostUpdate, error) {
	rows, err := s.q.ListPublishedPostUpdates(ctx, sql.NullTime{Time: at, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("查询已发布内容：%w", err)
	}
	out := make([]PostUpdate, len(rows))
	for i, r := range rows {
		out[i] = PostUpdate{ID: r.ID, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

// CategorySlugs 全部分类的 slug。
func (s *Service) CategorySlugs(ctx context.Context) ([]string, error) {
	slugs, err := s.q.ListCategorySlugs(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询分类：%w", err)
	}
	return slugs, nil
}

// TagSlugs 还有已发布内容的标签的 slug。
func (s *Service) TagSlugs(ctx context.Context, at time.Time) ([]string, error) {
	slugs, err := s.q.ListTagSlugsWithPosts(ctx, sql.NullTime{Time: at, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("查询标签：%w", err)
	}
	return slugs, nil
}
