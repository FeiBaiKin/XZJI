package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	platformdb "xzji/internal/platform/db"
)

// listRow 列表查询的一行：内容主表加资源扩展信息（文章没有资源信息，相应字段为 NULL）。
type listRow struct {
	ID           int64
	Type         string
	Title        string
	Summary      string
	CoverMediaID sql.NullInt64
	CategoryID   int64
	AuthorID     int64
	Status       string
	Pinned       bool
	ViewCount    int32
	CommentCount int32
	PublishedAt  sql.NullTime
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Platform     sql.NullString
	Version      sql.NullString
	FileSize     sql.NullString
	IconMediaID  sql.NullInt64
}

var listColumns = []string{
	"p.id", "p.type", "p.title", "p.summary", "p.cover_media_id", "p.category_id", "p.author_id", "p.status",
	"p.pinned", "p.view_count", "p.comment_count", "p.published_at", "p.created_at", "p.updated_at",
	"r.platform", "r.version", "r.file_size", "r.icon_media_id",
}

func (r *listRow) dest() []any {
	return []any{
		&r.ID, &r.Type, &r.Title, &r.Summary, &r.CoverMediaID, &r.CategoryID, &r.AuthorID, &r.Status,
		&r.Pinned, &r.ViewCount, &r.CommentCount, &r.PublishedAt, &r.CreatedAt, &r.UpdatedAt,
		&r.Platform, &r.Version, &r.FileSize, &r.IconMediaID,
	}
}

type sortOrder int

const (
	sortLatest sortOrder = iota
	sortViews
	sortID
)

type listFilter struct {
	public      bool
	typ         Type
	status      Status
	categoryIDs []int64
	tagID       int64
	platform    string
	keyword     string
	// keywordSummary 关键词同时匹配摘要（前台搜索）
	keywordSummary bool
	excludeID      int64
	sort           sortOrder
	page, size     int
}

func (s *Service) where(f listFilter) sq.And {
	w := sq.And{}
	switch {
	case f.public:
		w = append(w, sq.Eq{"p.status": StatusPublished}, sq.LtOrEq{"p.published_at": s.now().UTC()})
	case f.status != "":
		w = append(w, sq.Eq{"p.status": f.status})
	default:
		w = append(w, sq.NotEq{"p.status": StatusTrash})
	}
	if f.typ != "" {
		w = append(w, sq.Eq{"p.type": f.typ})
	}
	if len(f.categoryIDs) > 0 {
		w = append(w, sq.Eq{"p.category_id": f.categoryIDs})
	}
	if f.tagID > 0 {
		w = append(w, sq.Expr("EXISTS (SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id AND pt.tag_id = ?)", f.tagID))
	}
	if f.platform != "" {
		w = append(w, sq.Eq{"r.platform": f.platform})
	}
	if f.keyword != "" {
		like := platformdb.LikeContains(f.keyword)
		if f.keywordSummary {
			w = append(w, sq.Or{sq.Like{"p.title": like}, sq.Like{"p.summary": like}})
		} else {
			w = append(w, sq.Like{"p.title": like})
		}
	}
	if f.excludeID > 0 {
		w = append(w, sq.NotEq{"p.id": f.excludeID})
	}
	return w
}

// queryRows 按条件查询一页，withTotal 为 false 时不统计总数。
func (s *Service) queryRows(ctx context.Context, f listFilter, withTotal bool) ([]listRow, int64, error) {
	w := s.where(f)
	from := sq.Select().From("posts p").LeftJoin("resources r ON r.post_id = p.id").Where(w)

	var total int64
	if withTotal {
		query, args, err := from.Columns("COUNT(*)").ToSql()
		if err != nil {
			return nil, 0, fmt.Errorf("生成统计语句：%w", err)
		}
		if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("统计内容数：%w", err)
		}
		if total == 0 {
			return []listRow{}, 0, nil
		}
	}

	var order []string
	switch f.sort {
	case sortViews:
		order = []string{"p.view_count DESC", "p.published_at DESC", "p.id DESC"}
	case sortID:
		order = []string{"p.id DESC"}
	default:
		order = []string{"p.pinned DESC", "p.published_at DESC", "p.id DESC"}
	}
	query, args, err := from.Columns(listColumns...).OrderBy(order...).
		Limit(uint64(f.size)).Offset(uint64((f.page - 1) * f.size)).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("生成查询语句：%w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询内容列表：%w", err)
	}
	defer rows.Close()
	out := []listRow{}
	for rows.Next() {
		var r listRow
		if err := rows.Scan(r.dest()...); err != nil {
			return nil, 0, fmt.Errorf("读取内容列表：%w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取内容列表：%w", err)
	}
	return out, total, nil
}

// hydrate 批量补齐分类、标签、作者、封面和图标，每类各一次查询。
func (s *Service) hydrate(ctx context.Context, rows []listRow) ([]*Post, error) {
	out := make([]*Post, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]int64, len(rows))
	authorIDs := make([]int64, 0, len(rows))
	mediaIDs := make([]int64, 0, len(rows)*2)
	for i, r := range rows {
		ids[i] = r.ID
		authorIDs = append(authorIDs, r.AuthorID)
		mediaIDs = append(mediaIDs, r.CoverMediaID.Int64, r.IconMediaID.Int64)
	}

	cats, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询分类：%w", err)
	}
	catRef := make(map[int64]Ref, len(cats))
	for _, c := range cats {
		catRef[c.ID] = Ref{ID: c.ID, Name: c.Name, Slug: c.Slug}
	}
	tagRows, err := s.q.ListTagsByPostIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("查询内容标签：%w", err)
	}
	tags := make(map[int64][]Ref, len(rows))
	for _, t := range tagRows {
		tags[t.PostID] = append(tags[t.PostID], Ref{ID: t.ID, Name: t.Name, Slug: t.Slug})
	}
	names, err := s.users.Nicknames(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	medias, err := s.media.GetMany(ctx, mediaIDs)
	if err != nil {
		return nil, err
	}

	for i, r := range rows {
		p := &Post{
			ID: r.ID, Type: Type(r.Type), Title: r.Title, Summary: r.Summary,
			Category: catRef[r.CategoryID], Tags: tags[r.ID], Author: Author{ID: r.AuthorID, Nickname: names[r.AuthorID]},
			Status: Status(r.Status), Pinned: r.Pinned, ViewCount: int(r.ViewCount), CommentCount: int(r.CommentCount),
			PublishedAt: ptrTime(r.PublishedAt), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if p.Tags == nil {
			p.Tags = []Ref{}
		}
		if m, ok := medias[r.CoverMediaID.Int64]; ok {
			p.Cover = &m
		}
		if Type(r.Type) == TypeResource {
			p.Resource = &Resource{Platform: r.Platform.String, Version: r.Version.String, FileSize: r.FileSize.String}
			if m, ok := medias[r.IconMediaID.Int64]; ok {
				p.Resource.Icon = &m
			}
		}
		out[i] = p
	}
	return out, nil
}

// PostPage 分页结果，Page 和 PageSize 是规范化之后实际使用的值。
type PostPage struct {
	Posts    []*Post
	Total    int64
	Page     int
	PageSize int
}

func (s *Service) page(ctx context.Context, f listFilter) (PostPage, error) {
	rows, total, err := s.queryRows(ctx, f, true)
	if err != nil {
		return PostPage{}, err
	}
	posts, err := s.hydrate(ctx, rows)
	if err != nil {
		return PostPage{}, err
	}
	return PostPage{Posts: posts, Total: total, Page: f.page, PageSize: f.size}, nil
}

type PublicQuery struct {
	Type     Type
	Category string
	Tag      string
	Platform string
	Keyword  string
	// Sort 为 views 时按浏览量排序，否则置顶在前、按发布时间倒序
	Sort     string
	Page     int
	PageSize int
}

// categoryScope 把分类 slug 解析为分类自身及子分类的 ID。
func (s *Service) categoryScope(ctx context.Context, slug string) ([]int64, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询分类：%w", err)
	}
	for _, r := range rows {
		if r.Slug == slug {
			return descendantIDs(rows, r.ID), nil
		}
	}
	return nil, ErrCategoryNotFound
}

// ListPublished 前台列表，只返回已发布且到了发布时间的内容。
func (s *Service) ListPublished(ctx context.Context, in PublicQuery) (PostPage, error) {
	f := listFilter{public: true, typ: in.Type, platform: strings.TrimSpace(in.Platform), keywordSummary: true}
	f.page, f.size = normPage(in.Page, in.PageSize, 12, 50)
	f.keyword = strings.TrimSpace(in.Keyword)
	if in.Sort == "views" {
		f.sort = sortViews
	}
	if in.Category != "" {
		ids, err := s.categoryScope(ctx, in.Category)
		if err != nil {
			return PostPage{}, err
		}
		f.categoryIDs = ids
	}
	if in.Tag != "" {
		t, err := s.q.GetTagBySlug(ctx, in.Tag)
		if errors.Is(err, sql.ErrNoRows) {
			return PostPage{}, ErrTagNotFound
		}
		if err != nil {
			return PostPage{}, fmt.Errorf("查询标签：%w", err)
		}
		f.tagID = t.ID
	}
	return s.page(ctx, f)
}

// RelatedPosts 同分类下最新发布的其他内容。
func (s *Service) RelatedPosts(ctx context.Context, id int64, limit int) ([]*Post, error) {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return nil, err
	}
	if !s.visible(row) {
		return nil, ErrPostNotFound
	}
	if limit < 1 || limit > 12 {
		limit = 6
	}
	f := listFilter{public: true, categoryIDs: []int64{row.CategoryID}, excludeID: id, page: 1, size: limit}
	rows, _, err := s.queryRows(ctx, f, false)
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, rows)
}

type AdminQuery struct {
	Type       Type
	Status     Status
	CategoryID int64
	Keyword    string
	Page       int
	PageSize   int
}

// ListPosts 后台列表，不指定状态时不含回收站。
func (s *Service) ListPosts(ctx context.Context, in AdminQuery) (PostPage, error) {
	f := listFilter{typ: in.Type, status: in.Status, keyword: strings.TrimSpace(in.Keyword), sort: sortID}
	f.page, f.size = normPage(in.Page, in.PageSize, 20, 100)
	if in.CategoryID > 0 {
		rows, err := s.q.ListCategories(ctx)
		if err != nil {
			return PostPage{}, fmt.Errorf("查询分类：%w", err)
		}
		f.categoryIDs = descendantIDs(rows, in.CategoryID)
	}
	return s.page(ctx, f)
}

type PlatformCount struct {
	Name  string
	Count int64
}

// Platforms 已发布资源里出现过的平台及数量，可限定分类（含子分类）。
func (s *Service) Platforms(ctx context.Context, categorySlug string) ([]PlatformCount, error) {
	q := sq.Select("r.platform", "COUNT(*)").From("resources r").Join("posts p ON p.id = r.post_id").
		Where(sq.Eq{"p.status": StatusPublished}).Where(sq.LtOrEq{"p.published_at": s.now().UTC()}).
		Where(sq.NotEq{"r.platform": ""}).GroupBy("r.platform").OrderBy("COUNT(*) DESC", "r.platform")
	if categorySlug != "" {
		ids, err := s.categoryScope(ctx, categorySlug)
		if err != nil {
			return nil, err
		}
		q = q.Where(sq.Eq{"p.category_id": ids})
	}
	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("生成查询语句：%w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("统计资源平台：%w", err)
	}
	defer rows.Close()
	out := []PlatformCount{}
	for rows.Next() {
		var p PlatformCount
		if err := rows.Scan(&p.Name, &p.Count); err != nil {
			return nil, fmt.Errorf("读取资源平台：%w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlatformNames 已使用过的平台名称，供后台编辑时选择。
func (s *Service) PlatformNames(ctx context.Context) ([]string, error) {
	names, err := s.q.ListPlatformNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询资源平台：%w", err)
	}
	return names, nil
}
