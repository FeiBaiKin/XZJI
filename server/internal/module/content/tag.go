package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"xzji/internal/dbgen"
	platformdb "xzji/internal/platform/db"
)

const maxTagsPerPost = 10

type TagFilter struct {
	Keyword  string
	Page     int
	PageSize int
}

type TagPage struct {
	Tags     []Tag
	Total    int64
	Page     int
	PageSize int
}

func (s *Service) ListTags(ctx context.Context, f TagFilter) (TagPage, error) {
	f.Page, f.PageSize = normPage(f.Page, f.PageSize, 20, 100)
	var kw sql.NullString
	if k := strings.TrimSpace(f.Keyword); k != "" {
		kw = sql.NullString{String: platformdb.LikeContains(k), Valid: true}
	}
	rows, err := s.q.ListTagsWithCount(ctx, dbgen.ListTagsWithCountParams{
		Keyword: kw, Limit: int32(f.PageSize), Offset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return TagPage{}, fmt.Errorf("查询标签：%w", err)
	}
	total, err := s.q.CountTags(ctx, dbgen.CountTagsParams{Keyword: kw})
	if err != nil {
		return TagPage{}, fmt.Errorf("统计标签：%w", err)
	}
	out := TagPage{Tags: make([]Tag, len(rows)), Total: total, Page: f.Page, PageSize: f.PageSize}
	for i, r := range rows {
		out.Tags[i] = Tag{ID: r.ID, Name: r.Name, Slug: r.Slug, PostCount: r.PostCount, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// HotTags 按已发布内容数倒序。
func (s *Service) HotTags(ctx context.Context, limit int) ([]Tag, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.q.ListHotTags(ctx, dbgen.ListHotTagsParams{
		PublishedAt: sql.NullTime{Time: s.now().UTC(), Valid: true}, Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("查询热门标签：%w", err)
	}
	out := make([]Tag, len(rows))
	for i, r := range rows {
		out[i] = Tag{ID: r.ID, Name: r.Name, Slug: r.Slug, PostCount: r.PostCount}
	}
	return out, nil
}

// TagBySlug 前台标签页使用，内容数只算已发布的。
func (s *Service) TagBySlug(ctx context.Context, slug string) (Tag, error) {
	r, err := s.q.GetTagBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return Tag{}, ErrTagNotFound
	}
	if err != nil {
		return Tag{}, fmt.Errorf("查询标签：%w", err)
	}
	n, err := s.q.CountPublishedPostsByTag(ctx, dbgen.CountPublishedPostsByTagParams{
		TagID: r.ID, PublishedAt: sql.NullTime{Time: s.now().UTC(), Valid: true},
	})
	if err != nil {
		return Tag{}, fmt.Errorf("统计标签内容数：%w", err)
	}
	return Tag{ID: r.ID, Name: r.Name, Slug: r.Slug, PostCount: n, CreatedAt: r.CreatedAt}, nil
}

func (s *Service) getTag(ctx context.Context, q *dbgen.Queries, id int64) (Tag, error) {
	r, err := q.GetTag(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Tag{}, ErrTagNotFound
	}
	if err != nil {
		return Tag{}, fmt.Errorf("查询标签 %d：%w", id, err)
	}
	return Tag{ID: r.ID, Name: r.Name, Slug: r.Slug, CreatedAt: r.CreatedAt}, nil
}

func normTagName(name string) (string, error) {
	name, err := checkLen("标签名称", name, 32)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrNameRequired
	}
	return name, nil
}

// uniqueTagSlug 由名称生成 slug，被占用时依次加上 -2、-3 等后缀。
func uniqueTagSlug(ctx context.Context, q *dbgen.Queries, name string, selfID int64) (string, error) {
	base := tagSlugFrom(name)
	for i := 1; i <= 50; i++ {
		slug := base
		if i > 1 {
			slug = base + "-" + strconv.Itoa(i)
		}
		r, err := q.GetTagBySlug(ctx, slug)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && r.ID == selfID) {
			return slug, nil
		}
		if err != nil {
			return "", fmt.Errorf("检查标签 slug：%w", err)
		}
	}
	return "", ErrSlugTaken
}

func (s *Service) tagSlug(ctx context.Context, q *dbgen.Queries, name, slug string, selfID int64) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return uniqueTagSlug(ctx, q, name, selfID)
	}
	slug = strings.ToLower(slug)
	if !tagSlugOK(slug) {
		return "", ErrInvalidTagSlug
	}
	return slug, nil
}

// tagConflict 区分名称重复和 slug 重复。
func tagConflict(ctx context.Context, q *dbgen.Queries, name string, selfID int64) error {
	if r, err := q.GetTagByName(ctx, name); err == nil && r.ID != selfID {
		return ErrTagNameTaken
	}
	return ErrSlugTaken
}

func (s *Service) CreateTag(ctx context.Context, name, slug string) (Tag, error) {
	name, err := normTagName(name)
	if err != nil {
		return Tag{}, err
	}
	if slug, err = s.tagSlug(ctx, s.q, name, slug, 0); err != nil {
		return Tag{}, err
	}
	id, err := s.q.CreateTag(ctx, dbgen.CreateTagParams{Name: name, Slug: slug})
	if platformdb.IsDuplicate(err) {
		return Tag{}, tagConflict(ctx, s.q, name, 0)
	}
	if err != nil {
		return Tag{}, fmt.Errorf("新建标签：%w", err)
	}
	return s.getTag(ctx, s.q, id)
}

func (s *Service) UpdateTag(ctx context.Context, id int64, name, slug string) (Tag, error) {
	if _, err := s.getTag(ctx, s.q, id); err != nil {
		return Tag{}, err
	}
	name, err := normTagName(name)
	if err != nil {
		return Tag{}, err
	}
	if slug, err = s.tagSlug(ctx, s.q, name, slug, id); err != nil {
		return Tag{}, err
	}
	err = s.q.UpdateTag(ctx, dbgen.UpdateTagParams{Name: name, Slug: slug, ID: id})
	if platformdb.IsDuplicate(err) {
		return Tag{}, tagConflict(ctx, s.q, name, id)
	}
	if err != nil {
		return Tag{}, fmt.Errorf("修改标签 %d：%w", id, err)
	}
	return s.getTag(ctx, s.q, id)
}

// DeleteTag 同时解除与内容的关联。
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	if _, err := s.getTag(ctx, s.q, id); err != nil {
		return err
	}
	return platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.DeletePostTagsByTag(ctx, id); err != nil {
			return fmt.Errorf("解除标签关联：%w", err)
		}
		if err := q.DeleteTag(ctx, id); err != nil {
			return fmt.Errorf("删除标签 %d：%w", id, err)
		}
		return nil
	})
}

// normTagNames 去掉空白和重复（不区分大小写，与数据库排序规则一致），最多 10 个。
func normTagNames(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n, err := checkLen("标签名称", n, 32)
		if err != nil {
			return nil, err
		}
		k := strings.ToLower(n)
		if n == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, n)
	}
	if len(out) > maxTagsPerPost {
		return nil, ErrTooManyTags
	}
	return out, nil
}

// resolveTags 在事务内把标签名转换为 ID，不存在的自动创建，返回顺序与输入一致。
func (s *Service) resolveTags(ctx context.Context, q *dbgen.Queries, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}
	rows, err := q.ListTagsByNames(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("查询标签：%w", err)
	}
	existing := make(map[string]int64, len(rows))
	for _, r := range rows {
		existing[strings.ToLower(r.Name)] = r.ID
	}
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		if id, ok := existing[strings.ToLower(name)]; ok {
			ids = append(ids, id)
			continue
		}
		// 排序规则对全角、重音等也做了折叠，小写匹配不上时按名称再查一次
		if r, err := q.GetTagByName(ctx, name); err == nil {
			ids = append(ids, r.ID)
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("查询标签：%w", err)
		}
		slug, err := uniqueTagSlug(ctx, q, name, 0)
		if err != nil {
			return nil, err
		}
		id, err := q.CreateTag(ctx, dbgen.CreateTagParams{Name: name, Slug: slug})
		if err != nil {
			return nil, fmt.Errorf("新建标签 %s：%w", name, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
