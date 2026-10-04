// Package seo 汇总 sitemap 需要的数据：已发布内容、分类和有内容的标签。
package seo

import (
	"context"
	"time"

	"xzji/internal/module/content"
)

type Data struct {
	Posts      []content.PostUpdate
	Categories []string
	Tags       []string
}

type Service struct {
	content *content.Service
	now     func() time.Time
}

func NewService(c *content.Service) *Service {
	return &Service{content: c, now: time.Now}
}

// Sitemap 返回 sitemap 需要的数据。1 万篇这个量级一次返回即可；
// 站点超过 5 万条 URL 时再拆成多个 sitemap 和索引文件（见 docs/m5-seo-performance.md 4.5）。
func (s *Service) Sitemap(ctx context.Context) (Data, error) {
	at := s.now().UTC()
	posts, err := s.content.PublishedUpdates(ctx, at)
	if err != nil {
		return Data{}, err
	}
	categories, err := s.content.CategorySlugs(ctx)
	if err != nil {
		return Data{}, err
	}
	tags, err := s.content.TagSlugs(ctx, at)
	if err != nil {
		return Data{}, err
	}
	return Data{Posts: posts, Categories: categories, Tags: tags}, nil
}
