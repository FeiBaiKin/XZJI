package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"xzji/internal/dbgen"
	"xzji/internal/module/media"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
)

var ErrStatusChanged = errs.New(errs.KindConflict, "STATUS_CHANGED", "内容状态已经变化，请刷新后重试")

const (
	maxAttrs     = 20
	maxChangelog = 50
)

type PostInput struct {
	Type           Type
	Title          string
	Summary        string
	ContentHTML    string
	CoverID        *int64
	CategoryID     int64
	Tags           []string
	Status         Status
	Pinned         bool
	PublishedAt    *time.Time
	SEOTitle       string
	SEOKeywords    string
	SEODescription string
	Resource       *ResourceInput
}

type ResourceInput struct {
	Platform  string
	Version   string
	FileSize  string
	IconID    *int64
	Attrs     []Attr
	Changelog []ChangelogEntry
}

// postValues 校验和规范化后的待写入数据。
type postValues struct {
	post     dbgen.CreatePostParams
	tags     []string
	resource *dbgen.UpsertResourceParams
}

func (s *Service) preparePost(ctx context.Context, in PostInput, existing *dbgen.Post) (postValues, error) {
	var v postValues
	p := &v.post
	var err error

	if in.Type != TypeArticle && in.Type != TypeResource {
		return v, ErrInvalidType
	}
	p.Type = string(in.Type)
	if in.Status != StatusDraft && in.Status != StatusPublished {
		return v, ErrInvalidStatus
	}
	p.Status = string(in.Status)
	if p.Title, err = checkLen("标题", in.Title, 255); err != nil {
		return v, err
	}
	if p.Title == "" {
		return v, ErrTitleRequired
	}
	if _, err := s.GetCategory(ctx, in.CategoryID); err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return v, errs.Invalid("CATEGORY_REQUIRED", "请选择分类")
		}
		return v, err
	}
	p.CategoryID = in.CategoryID

	body, err := processHTML(in.ContentHTML, s.media.BaseURL())
	if err != nil {
		return v, err
	}
	p.ContentHtml = body.HTML
	if body.TOC == nil {
		body.TOC = []TOCItem{}
	}
	if p.Toc, err = json.Marshal(body.TOC); err != nil {
		return v, fmt.Errorf("编码目录：%w", err)
	}
	if p.Summary, err = checkLen("摘要", in.Summary, 500); err != nil {
		return v, err
	}
	if p.Summary == "" {
		p.Summary, p.SummaryAuto = autoSummary(body.Text), true
	}
	if v.tags, err = normTagNames(in.Tags); err != nil {
		return v, err
	}
	if p.SeoTitle, err = checkLen("SEO 标题", in.SEOTitle, 255); err != nil {
		return v, err
	}
	if p.SeoKeywords, err = checkLen("SEO 关键词", in.SEOKeywords, 255); err != nil {
		return v, err
	}
	if p.SeoDescription, err = checkLen("SEO 描述", in.SEODescription, 500); err != nil {
		return v, err
	}
	p.Pinned = in.Pinned
	p.CoverMediaID = nullInt64(in.CoverID)

	switch {
	case in.PublishedAt != nil:
		p.PublishedAt = sql.NullTime{Time: in.PublishedAt.UTC(), Valid: true}
	case in.Status == StatusPublished && existing != nil && existing.PublishedAt.Valid:
		p.PublishedAt = existing.PublishedAt
	case in.Status == StatusPublished:
		p.PublishedAt = sql.NullTime{Time: s.now().UTC(), Valid: true}
	}

	mediaIDs := []int64{p.CoverMediaID.Int64}
	if in.Type == TypeResource {
		r := in.Resource
		if r == nil {
			r = &ResourceInput{}
		}
		if v.resource, err = prepareResource(r); err != nil {
			return v, err
		}
		mediaIDs = append(mediaIDs, v.resource.IconMediaID.Int64)
	}
	found, err := s.media.GetMany(ctx, mediaIDs)
	if err != nil {
		return v, err
	}
	for _, id := range mediaIDs {
		if _, ok := found[id]; id > 0 && !ok {
			return v, media.ErrNotFound
		}
	}
	return v, nil
}

func prepareResource(in *ResourceInput) (*dbgen.UpsertResourceParams, error) {
	r := &dbgen.UpsertResourceParams{IconMediaID: nullInt64(in.IconID)}
	var err error
	if r.Platform, err = checkLen("平台", in.Platform, 32); err != nil {
		return nil, err
	}
	if r.Version, err = checkLen("版本号", in.Version, 64); err != nil {
		return nil, err
	}
	if r.FileSize, err = checkLen("文件大小", in.FileSize, 32); err != nil {
		return nil, err
	}

	attrs := []Attr{}
	for _, a := range in.Attrs {
		if a.Name, err = checkLen("属性名", a.Name, 32); err != nil {
			return nil, err
		}
		if a.Value, err = checkLen("属性值", a.Value, 255); err != nil {
			return nil, err
		}
		if a.Name == "" && a.Value == "" {
			continue
		}
		if a.Name == "" {
			return nil, errs.Invalid("ATTR_NAME_REQUIRED", "请填写属性名")
		}
		attrs = append(attrs, a)
	}
	if len(attrs) > maxAttrs {
		return nil, errs.Invalid("TOO_MANY_ATTRS", "自定义属性最多 20 项")
	}

	logs := []ChangelogEntry{}
	for _, l := range in.Changelog {
		if l.Version, err = checkLen("版本号", l.Version, 64); err != nil {
			return nil, err
		}
		if l.Note, err = checkLen("更新说明", l.Note, 2000); err != nil {
			return nil, err
		}
		l.Date = strings.TrimSpace(l.Date)
		if l.Version == "" && l.Date == "" && l.Note == "" {
			continue
		}
		if l.Date != "" {
			if _, err := time.Parse(time.DateOnly, l.Date); err != nil {
				return nil, ErrInvalidDate
			}
		}
		logs = append(logs, l)
	}
	if len(logs) > maxChangelog {
		return nil, errs.Invalid("TOO_MANY_CHANGELOG", "更新日志最多 50 条")
	}

	if r.Attrs, err = json.Marshal(attrs); err != nil {
		return nil, fmt.Errorf("编码自定义属性：%w", err)
	}
	if r.Changelog, err = json.Marshal(logs); err != nil {
		return nil, fmt.Errorf("编码更新日志：%w", err)
	}
	return r, nil
}

// saveRelations 写入标签和资源信息，在事务内调用。
func (s *Service) saveRelations(ctx context.Context, q *dbgen.Queries, postID int64, v postValues) error {
	tagIDs, err := s.resolveTags(ctx, q, v.tags)
	if err != nil {
		return err
	}
	if err := q.DeletePostTags(ctx, postID); err != nil {
		return fmt.Errorf("清除内容标签：%w", err)
	}
	seen := map[int64]bool{}
	for i, id := range tagIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if err := q.AddPostTag(ctx, dbgen.AddPostTagParams{PostID: postID, TagID: id, Sort: int32(i)}); err != nil {
			return fmt.Errorf("关联标签：%w", err)
		}
	}
	if v.resource == nil {
		if err := q.DeleteResource(ctx, postID); err != nil {
			return fmt.Errorf("清除资源信息：%w", err)
		}
		return nil
	}
	v.resource.PostID = postID
	if err := q.UpsertResource(ctx, *v.resource); err != nil {
		return fmt.Errorf("保存资源信息：%w", err)
	}
	return nil
}

func (s *Service) CreatePost(ctx context.Context, authorID int64, in PostInput) (*Post, error) {
	v, err := s.preparePost(ctx, in, nil)
	if err != nil {
		return nil, err
	}
	v.post.AuthorID = authorID
	var id int64
	err = platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if id, err = q.CreatePost(ctx, v.post); err != nil {
			return fmt.Errorf("新建内容：%w", err)
		}
		return s.saveRelations(ctx, q, id, v)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPost(ctx, id)
}

func (s *Service) UpdatePost(ctx context.Context, id int64, in PostInput) (*Post, error) {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return nil, err
	}
	if Status(row.Status) == StatusTrash {
		return nil, ErrInTrash
	}
	v, err := s.preparePost(ctx, in, &row)
	if err != nil {
		return nil, err
	}
	p := v.post
	err = platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.UpdatePost(ctx, dbgen.UpdatePostParams{
			Type: p.Type, Title: p.Title, Summary: p.Summary, SummaryAuto: p.SummaryAuto, ContentHtml: p.ContentHtml, Toc: p.Toc,
			CoverMediaID: p.CoverMediaID, CategoryID: p.CategoryID, Status: p.Status, Pinned: p.Pinned,
			SeoTitle: p.SeoTitle, SeoKeywords: p.SeoKeywords, SeoDescription: p.SeoDescription,
			PublishedAt: p.PublishedAt, ID: id,
		}); err != nil {
			return fmt.Errorf("修改内容 %d：%w", id, err)
		}
		return s.saveRelations(ctx, q, id, v)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPost(ctx, id)
}

func (s *Service) getRow(ctx context.Context, id int64) (dbgen.Post, error) {
	row, err := s.q.GetPost(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrPostNotFound
	}
	if err != nil {
		return row, fmt.Errorf("查询内容 %d：%w", id, err)
	}
	return row, nil
}

func (s *Service) setStatus(ctx context.Context, id int64, from, to Status) error {
	n, err := s.q.UpdatePostStatus(ctx, dbgen.UpdatePostStatusParams{ID: id, FromStatus: string(from), ToStatus: string(to)})
	if err != nil {
		return fmt.Errorf("修改内容 %d 的状态：%w", id, err)
	}
	if n == 0 {
		return ErrStatusChanged
	}
	return nil
}

// TrashPost 移入回收站，已在回收站时直接返回。
func (s *Service) TrashPost(ctx context.Context, id int64) error {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return err
	}
	if Status(row.Status) == StatusTrash {
		return nil
	}
	return s.setStatus(ctx, id, Status(row.Status), StatusTrash)
}

// RestorePost 从回收站恢复为草稿。
func (s *Service) RestorePost(ctx context.Context, id int64) error {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return err
	}
	if Status(row.Status) != StatusTrash {
		return errs.Invalid("NOT_IN_TRASH", "这篇内容不在回收站里")
	}
	return s.setStatus(ctx, id, StatusTrash, StatusDraft)
}

// DeletePost 彻底删除回收站里的内容，并写入后台日志。
func (s *Service) DeletePost(ctx context.Context, adminID int64, ip string, id int64) error {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return err
	}
	if Status(row.Status) != StatusTrash {
		return ErrNotInTrash
	}
	return platformdb.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.DeletePostTags(ctx, id); err != nil {
			return fmt.Errorf("清除内容标签：%w", err)
		}
		if err := q.DeleteResource(ctx, id); err != nil {
			return fmt.Errorf("清除资源信息：%w", err)
		}
		if err := q.DeletePost(ctx, id); err != nil {
			return fmt.Errorf("删除内容 %d：%w", id, err)
		}
		return s.audit.Record(ctx, adminID, "post.delete", "post", id, map[string]string{"title": row.Title, "type": row.Type}, ip)
	})
}

// GetPost 后台查询单篇，任何状态都返回。
func (s *Service) GetPost(ctx context.Context, id int64) (*Post, error) {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.loadPost(ctx, row)
}

// GetPublishedPost 前台查询单篇，未发布、定时未到或在回收站的都视为不存在。
func (s *Service) GetPublishedPost(ctx context.Context, id int64) (*Post, error) {
	row, err := s.getRow(ctx, id)
	if err != nil {
		return nil, err
	}
	if !s.visible(row) {
		return nil, ErrPostNotFound
	}
	return s.loadPost(ctx, row)
}

func (s *Service) visible(row dbgen.Post) bool {
	return Status(row.Status) == StatusPublished && row.PublishedAt.Valid && !row.PublishedAt.Time.After(s.now())
}

func (s *Service) loadPost(ctx context.Context, row dbgen.Post) (*Post, error) {
	p := &listRow{
		ID: row.ID, Type: row.Type, Title: row.Title, Summary: row.Summary, CoverMediaID: row.CoverMediaID,
		CategoryID: row.CategoryID, AuthorID: row.AuthorID, Status: row.Status, Pinned: row.Pinned,
		ViewCount: row.ViewCount, CommentCount: row.CommentCount, PublishedAt: row.PublishedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	var res *dbgen.Resource
	if Type(row.Type) == TypeResource {
		r, err := s.q.GetResource(ctx, row.ID)
		switch {
		case err == nil:
			res = &r
		case errors.Is(err, sql.ErrNoRows):
			res = &dbgen.Resource{PostID: row.ID}
		default:
			return nil, fmt.Errorf("查询资源信息：%w", err)
		}
		p.Platform, p.Version, p.FileSize = sql.NullString{String: res.Platform, Valid: true}, sql.NullString{String: res.Version, Valid: true}, sql.NullString{String: res.FileSize, Valid: true}
		p.IconMediaID = res.IconMediaID
	}
	posts, err := s.hydrate(ctx, []listRow{*p})
	if err != nil {
		return nil, err
	}
	post := posts[0]
	post.ContentHTML = outputHTML(row.ContentHtml, s.media.BaseURL())
	post.TOC = []TOCItem{}
	if len(row.Toc) > 0 {
		if err := json.Unmarshal(row.Toc, &post.TOC); err != nil {
			return nil, fmt.Errorf("解析目录：%w", err)
		}
	}
	post.SEOTitle, post.SEOKeywords, post.SEODescription = row.SeoTitle, row.SeoKeywords, row.SeoDescription
	post.SummaryAuto = row.SummaryAuto
	if res != nil {
		post.Resource.Attrs = []Attr{}
		post.Resource.Changelog = []ChangelogEntry{}
		if len(res.Attrs) > 0 {
			if err := json.Unmarshal(res.Attrs, &post.Resource.Attrs); err != nil {
				return nil, fmt.Errorf("解析自定义属性：%w", err)
			}
		}
		if len(res.Changelog) > 0 {
			if err := json.Unmarshal(res.Changelog, &post.Resource.Changelog); err != nil {
				return nil, fmt.Errorf("解析更新日志：%w", err)
			}
		}
	}
	return post, nil
}
