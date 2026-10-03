package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"xzji/internal/dbgen"
	platformdb "xzji/internal/platform/db"
)

type CategoryInput struct {
	ParentID       *int64
	Name           string
	Slug           string
	Icon           string
	Color          string
	Description    string
	Sort           int
	SEOTitle       string
	SEODescription string
}

func categoryFromRow(r dbgen.Category) *Category {
	return &Category{
		ID: r.ID, ParentID: ptrInt64(r.ParentID), Name: r.Name, Slug: r.Slug, Icon: r.Icon, Color: r.Color,
		Description: r.Description, Sort: int(r.Sort), SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription,
	}
}

// buildTree 组装两级分类树，父分类的内容数包含子分类。行已按排序值排好。
func buildTree(rows []dbgen.Category, counts map[int64]int64) []*Category {
	byID := make(map[int64]*Category, len(rows))
	for _, r := range rows {
		c := categoryFromRow(r)
		c.PostCount = counts[r.ID]
		c.Children = []*Category{}
		byID[r.ID] = c
	}
	roots := []*Category{}
	for _, r := range rows {
		c := byID[r.ID]
		if p, ok := byID[r.ParentID.Int64]; r.ParentID.Valid && ok {
			p.Children = append(p.Children, c)
			p.PostCount += c.PostCount
		} else {
			roots = append(roots, c)
		}
	}
	return roots
}

// descendantIDs 返回分类自身及子分类的 ID。
func descendantIDs(rows []dbgen.Category, id int64) []int64 {
	ids := []int64{id}
	for _, r := range rows {
		if r.ParentID.Valid && r.ParentID.Int64 == id {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

// CategoryTree 前台使用的分类树，只统计已发布的内容。
func (s *Service) CategoryTree(ctx context.Context) ([]*Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询分类：%w", err)
	}
	cnt, err := s.q.CountPublishedPostsByCategory(ctx, sql.NullTime{Time: s.now().UTC(), Valid: true})
	if err != nil {
		return nil, fmt.Errorf("统计分类内容数：%w", err)
	}
	counts := make(map[int64]int64, len(cnt))
	for _, c := range cnt {
		counts[c.CategoryID] = c.PostCount
	}
	return buildTree(rows, counts), nil
}

// ListCategories 后台使用的平铺列表，内容数只算自身未删除的内容。
func (s *Service) ListCategories(ctx context.Context) ([]*Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询分类：%w", err)
	}
	cnt, err := s.q.CountActivePostsByCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计分类内容数：%w", err)
	}
	counts := make(map[int64]int64, len(cnt))
	for _, c := range cnt {
		counts[c.CategoryID] = c.PostCount
	}
	out := make([]*Category, len(rows))
	for i, r := range rows {
		out[i] = categoryFromRow(r)
		out[i].PostCount = counts[r.ID]
	}
	return out, nil
}

func (s *Service) GetCategory(ctx context.Context, id int64) (*Category, error) {
	r, err := s.q.GetCategory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询分类 %d：%w", id, err)
	}
	return categoryFromRow(r), nil
}

func (s *Service) validateCategory(ctx context.Context, id int64, in CategoryInput) (dbgen.CreateCategoryParams, error) {
	var p dbgen.CreateCategoryParams
	var err error
	if p.Name, err = checkLen("分类名称", in.Name, 32); err != nil {
		return p, err
	}
	if p.Name == "" {
		return p, ErrNameRequired
	}
	p.Slug = strings.TrimSpace(in.Slug)
	if len(p.Slug) > 64 || !slugRe.MatchString(p.Slug) {
		return p, ErrInvalidSlug
	}
	p.Icon = strings.TrimSpace(in.Icon)
	if !iconRe.MatchString(p.Icon) {
		return p, ErrInvalidIcon
	}
	p.Color = strings.TrimSpace(in.Color)
	if !colors[p.Color] {
		return p, ErrInvalidColor
	}
	if p.Description, err = checkLen("分类描述", in.Description, 255); err != nil {
		return p, err
	}
	if p.SeoTitle, err = checkLen("SEO 标题", in.SEOTitle, 255); err != nil {
		return p, err
	}
	if p.SeoDescription, err = checkLen("SEO 描述", in.SEODescription, 500); err != nil {
		return p, err
	}
	p.Sort = int32(in.Sort)

	if in.ParentID != nil && *in.ParentID > 0 {
		if *in.ParentID == id {
			return p, ErrInvalidParent
		}
		parent, err := s.GetCategory(ctx, *in.ParentID)
		if errors.Is(err, ErrCategoryNotFound) {
			return p, ErrInvalidParent
		}
		if err != nil {
			return p, err
		}
		if parent.ParentID != nil {
			return p, ErrInvalidParent
		}
		if id > 0 {
			n, err := s.q.CountChildCategories(ctx, sql.NullInt64{Int64: id, Valid: true})
			if err != nil {
				return p, fmt.Errorf("统计子分类：%w", err)
			}
			if n > 0 {
				return p, ErrInvalidParent
			}
		}
		p.ParentID = sql.NullInt64{Int64: parent.ID, Valid: true}
	}
	return p, nil
}

func (s *Service) CreateCategory(ctx context.Context, in CategoryInput) (*Category, error) {
	p, err := s.validateCategory(ctx, 0, in)
	if err != nil {
		return nil, err
	}
	id, err := s.q.CreateCategory(ctx, p)
	if platformdb.IsDuplicate(err) {
		return nil, ErrSlugTaken
	}
	if err != nil {
		return nil, fmt.Errorf("新建分类：%w", err)
	}
	return s.GetCategory(ctx, id)
}

func (s *Service) UpdateCategory(ctx context.Context, id int64, in CategoryInput) (*Category, error) {
	if _, err := s.GetCategory(ctx, id); err != nil {
		return nil, err
	}
	p, err := s.validateCategory(ctx, id, in)
	if err != nil {
		return nil, err
	}
	err = s.q.UpdateCategory(ctx, dbgen.UpdateCategoryParams{
		ParentID: p.ParentID, Name: p.Name, Slug: p.Slug, Icon: p.Icon, Color: p.Color, Description: p.Description,
		Sort: p.Sort, SeoTitle: p.SeoTitle, SeoDescription: p.SeoDescription, ID: id,
	})
	if platformdb.IsDuplicate(err) {
		return nil, ErrSlugTaken
	}
	if err != nil {
		return nil, fmt.Errorf("修改分类 %d：%w", id, err)
	}
	return s.GetCategory(ctx, id)
}

// DeleteCategory 有子分类或内容（包括回收站）时拒绝删除，避免内容失去分类。
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	if _, err := s.GetCategory(ctx, id); err != nil {
		return err
	}
	children, err := s.q.CountChildCategories(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return fmt.Errorf("统计子分类：%w", err)
	}
	posts, err := s.q.CountPostsInCategory(ctx, id)
	if err != nil {
		return fmt.Errorf("统计分类内容：%w", err)
	}
	if children > 0 || posts > 0 {
		return ErrCategoryInUse
	}
	if err := s.q.DeleteCategory(ctx, id); err != nil {
		return fmt.Errorf("删除分类 %d：%w", id, err)
	}
	return nil
}
