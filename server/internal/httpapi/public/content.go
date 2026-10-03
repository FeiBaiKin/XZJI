package public

import (
	"context"

	pubapi "xzji/internal/apigen/public"
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

func (h *Handler) GetHome(ctx context.Context, _ pubapi.GetHomeRequestObject) (pubapi.GetHomeResponseObject, error) {
	home, err := h.Home.Get(ctx)
	if err != nil {
		return nil, err
	}
	resp := pubapi.Home{Banners: make([]pubapi.Banner, len(home.Banners)), Notice: home.Notice}
	for i, b := range home.Banners {
		resp.Banners[i] = pubapi.Banner{
			Tag: b.Tag, Title: b.Title, Subtitle: b.Subtitle, ButtonText: b.ButtonText, Link: b.Link, Image: toImage(b.Image),
		}
	}
	return pubapi.GetHome200JSONResponse(resp), nil
}

func (h *Handler) ListCategories(ctx context.Context, _ pubapi.ListCategoriesRequestObject) (pubapi.ListCategoriesResponseObject, error) {
	tree, err := h.Content.CategoryTree(ctx)
	if err != nil {
		return nil, err
	}
	return pubapi.ListCategories200JSONResponse{Items: toCategoryNodes(tree)}, nil
}

func (h *Handler) ListPosts(ctx context.Context, req pubapi.ListPostsRequestObject) (pubapi.ListPostsResponseObject, error) {
	p := req.Params
	q := content.PublicQuery{
		Type: content.Type(deref(p.Type)), Category: deref(p.Category), Tag: deref(p.Tag), Platform: deref(p.Platform),
		Keyword: limitRunes(deref(p.Q), 50), Sort: string(deref(p.Sort)), Page: deref(p.Page), PageSize: deref(p.PageSize),
	}
	res, err := h.Content.ListPublished(ctx, q)
	if err != nil {
		return nil, err
	}
	return pubapi.ListPosts200JSONResponse{Items: toCards(res.Posts), Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) GetPost(ctx context.Context, req pubapi.GetPostRequestObject) (pubapi.GetPostResponseObject, error) {
	p, err := h.Content.GetPublishedPost(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	d := pubapi.PostDetail{
		Id: p.ID, Type: pubapi.PostType(p.Type), Title: p.Title, Summary: p.Summary, ContentHtml: p.ContentHTML,
		Toc: make([]pubapi.TocItem, len(p.TOC)), Cover: toImage(p.Cover), Category: toCategoryRef(p.Category),
		Tags: toTagRefs(p.Tags), Author: pubapi.Author{Id: p.Author.ID, Nickname: p.Author.Nickname},
		Pinned: p.Pinned, ViewCount: p.ViewCount, CommentCount: p.CommentCount,
		SeoTitle: p.SEOTitle, SeoKeywords: p.SEOKeywords, SeoDescription: p.SEODescription,
		PublishedAt: deref(p.PublishedAt), UpdatedAt: p.UpdatedAt,
	}
	for i, t := range p.TOC {
		d.Toc[i] = pubapi.TocItem{Id: t.ID, Text: t.Text, Level: t.Level}
	}
	if r := p.Resource; r != nil {
		d.Resource = &pubapi.ResourceDetail{
			Platform: r.Platform, Version: r.Version, FileSize: r.FileSize, Icon: toImage(r.Icon),
			Attrs: make([]pubapi.Attr, len(r.Attrs)), Changelog: make([]pubapi.ChangelogEntry, len(r.Changelog)),
		}
		for i, a := range r.Attrs {
			d.Resource.Attrs[i] = pubapi.Attr{Name: a.Name, Value: a.Value}
		}
		for i, l := range r.Changelog {
			d.Resource.Changelog[i] = pubapi.ChangelogEntry{Version: l.Version, Date: l.Date, Note: l.Note}
		}
	}
	return pubapi.GetPost200JSONResponse(d), nil
}

func (h *Handler) ListRelatedPosts(ctx context.Context, req pubapi.ListRelatedPostsRequestObject) (pubapi.ListRelatedPostsResponseObject, error) {
	posts, err := h.Content.RelatedPosts(ctx, req.Id, deref(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	return pubapi.ListRelatedPosts200JSONResponse{Items: toCards(posts)}, nil
}

func (h *Handler) ListTags(ctx context.Context, req pubapi.ListTagsRequestObject) (pubapi.ListTagsResponseObject, error) {
	tags, err := h.Content.HotTags(ctx, deref(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.Tag, len(tags))
	for i, t := range tags {
		items[i] = pubapi.Tag{Id: t.ID, Name: t.Name, Slug: t.Slug, PostCount: t.PostCount}
	}
	return pubapi.ListTags200JSONResponse{Items: items}, nil
}

func (h *Handler) GetTag(ctx context.Context, req pubapi.GetTagRequestObject) (pubapi.GetTagResponseObject, error) {
	t, err := h.Content.TagBySlug(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	return pubapi.GetTag200JSONResponse{Id: t.ID, Name: t.Name, Slug: t.Slug, PostCount: t.PostCount}, nil
}

func (h *Handler) ListPlatforms(ctx context.Context, req pubapi.ListPlatformsRequestObject) (pubapi.ListPlatformsResponseObject, error) {
	list, err := h.Content.Platforms(ctx, deref(req.Params.Category))
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.PlatformCount, len(list))
	for i, p := range list {
		items[i] = pubapi.PlatformCount{Name: p.Name, Count: p.Count}
	}
	return pubapi.ListPlatforms200JSONResponse{Items: items}, nil
}

func limitRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func toImage(m *media.Media) *pubapi.Image {
	if m == nil {
		return nil
	}
	img := &pubapi.Image{Url: m.URL, Width: m.Width, Height: m.Height}
	if m.Color != "" {
		img.Color = &m.Color
	}
	return img
}

func toCategoryRef(r content.Ref) pubapi.CategoryRef {
	return pubapi.CategoryRef{Id: r.ID, Name: r.Name, Slug: r.Slug}
}

func toTagRefs(refs []content.Ref) []pubapi.TagRef {
	out := make([]pubapi.TagRef, len(refs))
	for i, r := range refs {
		out[i] = pubapi.TagRef{Id: r.ID, Name: r.Name, Slug: r.Slug}
	}
	return out
}

func toCategoryNodes(cats []*content.Category) []pubapi.CategoryNode {
	out := make([]pubapi.CategoryNode, len(cats))
	for i, c := range cats {
		out[i] = pubapi.CategoryNode{
			Id: c.ID, Name: c.Name, Slug: c.Slug, Icon: c.Icon, Color: c.Color, Description: c.Description,
			SeoTitle: c.SEOTitle, SeoDescription: c.SEODescription, PostCount: c.PostCount,
			Children: toCategoryNodes(c.Children),
		}
	}
	return out
}

func toCards(posts []*content.Post) []pubapi.PostCard {
	out := make([]pubapi.PostCard, len(posts))
	for i, p := range posts {
		out[i] = pubapi.PostCard{
			Id: p.ID, Type: pubapi.PostType(p.Type), Title: p.Title, Summary: p.Summary, Cover: toImage(p.Cover),
			Category: toCategoryRef(p.Category), Tags: toTagRefs(p.Tags),
			Author: pubapi.Author{Id: p.Author.ID, Nickname: p.Author.Nickname},
			Pinned: p.Pinned, ViewCount: p.ViewCount, CommentCount: p.CommentCount, PublishedAt: deref(p.PublishedAt),
		}
		if r := p.Resource; r != nil {
			out[i].Resource = &pubapi.ResourceBrief{Platform: r.Platform, Version: r.Version, FileSize: r.FileSize, Icon: toImage(r.Icon)}
		}
	}
	return out
}
