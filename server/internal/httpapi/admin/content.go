package admin

import (
	"context"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/content"
	"xzji/internal/module/media"
)

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func toMedia(m *media.Media) *adminapi.Media {
	if m == nil {
		return nil
	}
	return &adminapi.Media{Id: m.ID, Url: m.URL, Width: m.Width, Height: m.Height, Mime: m.Mime, Size: m.Size, Color: m.Color}
}

// ---------- 分类 ----------

func toAdminCategory(c *content.Category) adminapi.AdminCategory {
	return adminapi.AdminCategory{
		Id: c.ID, ParentId: c.ParentID, Name: c.Name, Slug: c.Slug, Icon: c.Icon, Color: c.Color,
		Description: c.Description, Sort: c.Sort, SeoTitle: c.SEOTitle, SeoDescription: c.SEODescription, PostCount: c.PostCount,
	}
}

func categoryInput(b *adminapi.CategoryInput) content.CategoryInput {
	return content.CategoryInput{
		ParentID: b.ParentId, Name: b.Name, Slug: b.Slug, Icon: deref(b.Icon), Color: deref(b.Color),
		Description: deref(b.Description), Sort: deref(b.Sort), SEOTitle: deref(b.SeoTitle), SEODescription: deref(b.SeoDescription),
	}
}

func (h *Handler) ListCategories(ctx context.Context, _ adminapi.ListCategoriesRequestObject) (adminapi.ListCategoriesResponseObject, error) {
	cats, err := h.Content.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminCategory, len(cats))
	for i, c := range cats {
		items[i] = toAdminCategory(c)
	}
	return adminapi.ListCategories200JSONResponse{Items: items}, nil
}

func (h *Handler) CreateCategory(ctx context.Context, req adminapi.CreateCategoryRequestObject) (adminapi.CreateCategoryResponseObject, error) {
	c, err := h.Content.CreateCategory(ctx, categoryInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.CreateCategory201JSONResponse(toAdminCategory(c)), nil
}

func (h *Handler) UpdateCategory(ctx context.Context, req adminapi.UpdateCategoryRequestObject) (adminapi.UpdateCategoryResponseObject, error) {
	c, err := h.Content.UpdateCategory(ctx, req.Id, categoryInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateCategory200JSONResponse(toAdminCategory(c)), nil
}

func (h *Handler) DeleteCategory(ctx context.Context, req adminapi.DeleteCategoryRequestObject) (adminapi.DeleteCategoryResponseObject, error) {
	if err := h.Content.DeleteCategory(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteCategory204Response{}, nil
}

// ---------- 标签 ----------

func toAdminTag(t content.Tag) adminapi.AdminTag {
	return adminapi.AdminTag{Id: t.ID, Name: t.Name, Slug: t.Slug, PostCount: t.PostCount, CreatedAt: t.CreatedAt}
}

func (h *Handler) ListTags(ctx context.Context, req adminapi.ListTagsRequestObject) (adminapi.ListTagsResponseObject, error) {
	p := req.Params
	f := content.TagFilter{Keyword: deref(p.Keyword), Page: deref(p.Page), PageSize: deref(p.PageSize)}
	res, err := h.Content.ListTags(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminTag, len(res.Tags))
	for i, t := range res.Tags {
		items[i] = toAdminTag(t)
	}
	return adminapi.ListTags200JSONResponse{Items: items, Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) CreateTag(ctx context.Context, req adminapi.CreateTagRequestObject) (adminapi.CreateTagResponseObject, error) {
	t, err := h.Content.CreateTag(ctx, req.Body.Name, deref(req.Body.Slug))
	if err != nil {
		return nil, err
	}
	return adminapi.CreateTag201JSONResponse(toAdminTag(t)), nil
}

func (h *Handler) UpdateTag(ctx context.Context, req adminapi.UpdateTagRequestObject) (adminapi.UpdateTagResponseObject, error) {
	t, err := h.Content.UpdateTag(ctx, req.Id, req.Body.Name, deref(req.Body.Slug))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateTag200JSONResponse(toAdminTag(t)), nil
}

func (h *Handler) DeleteTag(ctx context.Context, req adminapi.DeleteTagRequestObject) (adminapi.DeleteTagResponseObject, error) {
	if err := h.Content.DeleteTag(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteTag204Response{}, nil
}

// ---------- 内容 ----------

func postInput(b *adminapi.PostInput) content.PostInput {
	in := content.PostInput{
		Type: content.Type(b.Type), Title: b.Title, Summary: deref(b.Summary), ContentHTML: b.ContentHtml,
		CoverID: b.CoverId, CategoryID: b.CategoryId, Tags: deref(b.Tags), Status: content.Status(b.Status),
		Pinned: deref(b.Pinned), PublishedAt: b.PublishedAt,
		Price: deref(b.Price), MemberFreeRank: deref(b.MemberFreeRank), MemberDiscount: deref(b.MemberDiscount),
		SEOTitle: deref(b.SeoTitle), SEOKeywords: deref(b.SeoKeywords), SEODescription: deref(b.SeoDescription),
	}
	if r := b.Resource; r != nil {
		in.Resource = &content.ResourceInput{
			Platform: deref(r.Platform), Version: deref(r.Version), FileSize: deref(r.FileSize), IconID: r.IconId,
			DownloadCond: deref(r.DownloadCond),
		}
		for _, l := range deref(r.Links) {
			in.Resource.Links = append(in.Resource.Links, content.Link{
				ID: deref(l.Id), Name: l.Name, URL: l.Url, ExtractCode: deref(l.ExtractCode),
				UnzipPassword: deref(l.UnzipPassword), Status: string(deref(l.Status)),
			})
		}
		for _, a := range deref(r.Attrs) {
			in.Resource.Attrs = append(in.Resource.Attrs, content.Attr{Name: a.Name, Value: a.Value})
		}
		for _, l := range deref(r.Changelog) {
			in.Resource.Changelog = append(in.Resource.Changelog, content.ChangelogEntry{Version: l.Version, Date: l.Date, Note: l.Note})
		}
	}
	return in
}

func toAdminPost(p *content.Post) adminapi.AdminPost {
	out := adminapi.AdminPost{
		Id: p.ID, Type: adminapi.PostType(p.Type), Title: p.Title, Summary: p.Summary, ContentHtml: p.EditorHTML(),
		Price: p.Price, MemberFreeRank: p.MemberFreeRank, MemberDiscount: p.MemberDiscount,
		Cover: toMedia(p.Cover), CategoryId: p.Category.ID, Tags: make([]string, len(p.Tags)),
		Status: adminapi.PostStatus(p.Status), Pinned: p.Pinned,
		SeoTitle: p.SEOTitle, SeoKeywords: p.SEOKeywords, SeoDescription: p.SEODescription,
		PublishedAt: p.PublishedAt, Author: adminapi.Author{Id: p.Author.ID, Nickname: p.Author.Nickname},
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	// 自动截取的摘要在编辑页显示为空，保存时继续按正文重新生成
	if p.SummaryAuto {
		out.Summary = ""
	}
	for i, t := range p.Tags {
		out.Tags[i] = t.Name
	}
	if r := p.Resource; r != nil {
		out.Resource = &adminapi.AdminResource{
			Platform: r.Platform, Version: r.Version, FileSize: r.FileSize, Icon: toMedia(r.Icon),
			Attrs: make([]adminapi.Attr, len(r.Attrs)), Changelog: make([]adminapi.ChangelogEntry, len(r.Changelog)),
			DownloadCond: r.DownloadCond, Links: make([]adminapi.AdminLink, len(r.Links)),
		}
		for i, l := range r.Links {
			out.Resource.Links[i] = adminapi.AdminLink{
				Id: l.ID, Name: l.Name, Url: l.URL, ExtractCode: l.ExtractCode, UnzipPassword: l.UnzipPassword, Status: adminapi.LinkStatus(l.Status),
			}
		}
		for i, a := range r.Attrs {
			out.Resource.Attrs[i] = adminapi.Attr{Name: a.Name, Value: a.Value}
		}
		for i, l := range r.Changelog {
			out.Resource.Changelog[i] = adminapi.ChangelogEntry{Version: l.Version, Date: l.Date, Note: l.Note}
		}
	}
	return out
}

func (h *Handler) ListPosts(ctx context.Context, req adminapi.ListPostsRequestObject) (adminapi.ListPostsResponseObject, error) {
	p := req.Params
	res, err := h.Content.ListPosts(ctx, content.AdminQuery{
		Type: content.Type(deref(p.Type)), Status: content.Status(deref(p.Status)), CategoryID: deref(p.CategoryId),
		Keyword: deref(p.Keyword), Page: deref(p.Page), PageSize: deref(p.PageSize),
	})
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminPostItem, len(res.Posts))
	for i, p := range res.Posts {
		items[i] = adminapi.AdminPostItem{
			Id: p.ID, Type: adminapi.PostType(p.Type), Title: p.Title, Status: adminapi.PostStatus(p.Status), Pinned: p.Pinned,
			Category: adminapi.CategoryRef{Id: p.Category.ID, Name: p.Category.Name, Slug: p.Category.Slug},
			Author:   adminapi.Author{Id: p.Author.ID, Nickname: p.Author.Nickname}, Cover: toMedia(p.Cover),
			ViewCount: p.ViewCount, CommentCount: p.CommentCount, PublishedAt: p.PublishedAt,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		}
	}
	return adminapi.ListPosts200JSONResponse{Items: items, Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) CreatePost(ctx context.Context, req adminapi.CreatePostRequestObject) (adminapi.CreatePostResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	p, err := h.Content.CreatePost(ctx, id.UserID, postInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.CreatePost201JSONResponse(toAdminPost(p)), nil
}

func (h *Handler) GetPost(ctx context.Context, req adminapi.GetPostRequestObject) (adminapi.GetPostResponseObject, error) {
	p, err := h.Content.GetPost(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return adminapi.GetPost200JSONResponse(toAdminPost(p)), nil
}

func (h *Handler) UpdatePost(ctx context.Context, req adminapi.UpdatePostRequestObject) (adminapi.UpdatePostResponseObject, error) {
	p, err := h.Content.UpdatePost(ctx, req.Id, postInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdatePost200JSONResponse(toAdminPost(p)), nil
}

func (h *Handler) DeletePost(ctx context.Context, req adminapi.DeletePostRequestObject) (adminapi.DeletePostResponseObject, error) {
	var err error
	if deref(req.Params.Permanent) {
		id, _ := authn.FromContext(ctx)
		err = h.Content.DeletePost(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.Id)
	} else {
		err = h.Content.TrashPost(ctx, req.Id)
	}
	if err != nil {
		return nil, err
	}
	return adminapi.DeletePost204Response{}, nil
}

func (h *Handler) RestorePost(ctx context.Context, req adminapi.RestorePostRequestObject) (adminapi.RestorePostResponseObject, error) {
	if err := h.Content.RestorePost(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.RestorePost204Response{}, nil
}

func (h *Handler) ListPlatforms(ctx context.Context, _ adminapi.ListPlatformsRequestObject) (adminapi.ListPlatformsResponseObject, error) {
	names, err := h.Content.PlatformNames(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.ListPlatforms200JSONResponse{Items: names}, nil
}
