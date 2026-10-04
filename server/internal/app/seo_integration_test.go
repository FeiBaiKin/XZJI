package app

import (
	"testing"
	"time"
)

// sitemap 只收录已发布且已到发布时间的公开内容，草稿、回收站和定时未到的都不收录。
func TestSitemapExcludesUnpublished(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	cat := testCategory(t, a, "pc")

	r := a.post("/admin-api/v1/posts", map[string]any{
		"type": "article", "title": "公开的文章", "categoryId": cat, "status": "published",
		"contentHtml": "<p>正文</p>", "tags": []string{"建站"},
	})
	expect(t, r, 201, "")
	published := num(r.body["id"])

	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	r = a.post("/admin-api/v1/posts", map[string]any{
		"type": "article", "title": "定时未到", "categoryId": cat, "status": "published",
		"publishedAt": future, "contentHtml": "<p>稍后</p>",
	})
	expect(t, r, 201, "")
	scheduled := num(r.body["id"])

	r = a.post("/admin-api/v1/posts", map[string]any{
		"type": "article", "title": "草稿", "categoryId": cat, "status": "draft", "contentHtml": "<p>草稿</p>",
	})
	expect(t, r, 201, "")
	draft := num(r.body["id"])

	trashed := publishedPost(t, a, cat, "回收站里的")
	expect(t, a.del("/admin-api/v1/posts/"+itoa(trashed)), 204, "")

	res := e.browser(t).get("/api/v1/seo/sitemap")
	expect(t, res, 200, "")
	ids := map[int64]bool{}
	for _, it := range res.body["posts"].([]any) {
		ids[num(it.(map[string]any)["id"])] = true
	}
	if !ids[published] {
		t.Error("已发布的内容应出现在 sitemap 里")
	}
	for name, postID := range map[string]int64{"定时未到": scheduled, "草稿": draft, "回收站": trashed} {
		if ids[postID] {
			t.Errorf("%s 的内容不该出现在 sitemap 里", name)
		}
	}
	if cats := res.body["categories"].([]any); len(cats) != 1 || cats[0] != "pc" {
		t.Errorf("分类 slug 不对：%v", cats)
	}
	if tags := res.body["tags"].([]any); len(tags) != 1 || tags[0] != "建站" {
		t.Errorf("标签 slug 不对：%v", tags)
	}
}
