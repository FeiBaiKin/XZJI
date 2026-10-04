package app

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// postStats 直接读库里的浏览量和更新时间。
func (e *env) postStats(t *testing.T, id int64) (int, time.Time) {
	t.Helper()
	var views int
	var updated time.Time
	if err := e.db.QueryRow(`SELECT view_count, updated_at FROM posts WHERE id = ?`, id).Scan(&views, &updated); err != nil {
		t.Fatal(err)
	}
	return views, updated
}

// testCategory 建一个分类，返回 ID。
func testCategory(t *testing.T, a *browser, slug string) int64 {
	t.Helper()
	r := a.post("/admin-api/v1/categories", map[string]any{"name": "电脑软件", "slug": slug})
	expect(t, r, 201, "")
	return num(r.body["id"])
}

// publishedPost 在指定分类下发布一篇内容，返回内容 ID。
func publishedPost(t *testing.T, a *browser, categoryID int64, title string) int64 {
	t.Helper()
	r := a.post("/admin-api/v1/posts", map[string]any{
		"type": "article", "title": title, "categoryId": categoryID, "status": "published",
		"contentHtml": "<p>正文</p>",
	})
	expect(t, r, 201, "")
	return num(r.body["id"])
}

// 上报浏览量：同一 IP 只计一次，Flush 后入库，且不改变 updated_at。
func TestPostViewReport(t *testing.T) {
	e := newEnv(t)
	admin := e.adminBrowser(t)
	postID := publishedPost(t, admin, testCategory(t, admin, "pc"), "浏览量测试")
	before, beforeUpdated := e.postStats(t, postID)

	visitor := e.browser(t)
	path := fmt.Sprintf("/api/v1/posts/%d/view", postID)
	expect(t, visitor.post(path, nil), 204, "")
	expect(t, visitor.post(path, nil), 204, "") // 同一 IP 重复上报，只计一次
	// 换一个 IP（测试里 127.0.0.1 是受信代理，X-Real-IP 生效）
	if r := visitor.do(http.MethodPost, path, nil, true, map[string]string{"X-Real-IP": "10.0.0.9"}); r.status != 204 {
		t.Fatalf("第二个 IP 上报返回 %d，期望 204", r.status)
	}
	// 不存在的内容同样返回 204，不报错也不影响计数
	if r := visitor.do(http.MethodPost, "/api/v1/posts/999999/view", nil, true, nil); r.status != 204 {
		t.Fatalf("不存在的内容上报返回 %d，期望 204", r.status)
	}

	if err := e.svc.Content.FlushViews(context.Background()); err != nil {
		t.Fatalf("Flush 出错：%v", err)
	}
	views, updated := e.postStats(t, postID)
	if views != before+2 {
		t.Errorf("浏览量 = %d，期望 %d", views, before+2)
	}
	if !updated.Equal(beforeUpdated) {
		t.Errorf("updated_at 被浏览量改动：%v → %v", beforeUpdated, updated)
	}

	// Flush 后内存已清空，再刷一次不会重复计数
	if err := e.svc.Content.FlushViews(context.Background()); err != nil {
		t.Fatalf("再次 Flush 出错：%v", err)
	}
	if views, _ := e.postStats(t, postID); views != before+2 {
		t.Errorf("重复 Flush 后浏览量 = %d，期望 %d", views, before+2)
	}
}
