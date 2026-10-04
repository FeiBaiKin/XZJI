package public

import (
	"context"

	pubapi "xzji/internal/apigen/public"
)

func (h *Handler) GetSitemap(ctx context.Context, _ pubapi.GetSitemapRequestObject) (pubapi.GetSitemapResponseObject, error) {
	data, err := h.Seo.Sitemap(ctx)
	if err != nil {
		return nil, err
	}
	posts := make([]pubapi.SitemapPost, len(data.Posts))
	for i, p := range data.Posts {
		posts[i] = pubapi.SitemapPost{Id: p.ID, UpdatedAt: p.UpdatedAt}
	}
	return pubapi.GetSitemap200JSONResponse(pubapi.SitemapData{
		Posts: posts, Categories: data.Categories, Tags: data.Tags,
	}), nil
}
