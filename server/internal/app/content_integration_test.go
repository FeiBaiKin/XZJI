package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"xzji/internal/module/user"
)

func (b *browser) upload(path string, data []byte) resp {
	b.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "icon.png")
	_, _ = fw.Write(data)
	_ = w.Close()
	return b.send(http.MethodPost, path, &buf, true, map[string]string{"Content-Type": w.FormDataContentType()})
}

func (b *browser) put(path string, body any) resp { return b.do(http.MethodPut, path, body, true, nil) }
func (b *browser) del(path string) resp           { return b.do(http.MethodDelete, path, nil, true, nil) }

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = []uint8{0x2F, 0x6B, 0xFF, 0xFF}[i%4]
	}
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *env) adminBrowser(t *testing.T) *browser {
	t.Helper()
	if _, err := e.svc.Users.Create(context.Background(), user.CreateInput{
		Email: "editor@example.com", Password: "edit12345", Nickname: "编辑部", Role: user.RoleAdmin,
	}); err != nil {
		t.Fatal(err)
	}
	a := e.browser(t)
	a.origin = e.srv.URL
	expect(t, a.post("/admin-api/v1/auth/login", map[string]any{"email": "editor@example.com", "password": "edit12345"}), 200, "")
	return a
}

func id(r resp) string { return itoa(int64(r.body["id"].(float64))) }

func items(t *testing.T, r resp) []map[string]any {
	t.Helper()
	raw, ok := r.body["items"].([]any)
	if !ok {
		t.Fatalf("响应里没有 items：%v", r.body)
	}
	out := make([]map[string]any, len(raw))
	for i, v := range raw {
		out[i] = v.(map[string]any)
	}
	return out
}

func TestContentPublishFlow(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	visitor := e.browser(t)

	// 上传图片：按内容判断类型，假冒的图片被拒绝
	expect(t, a.upload("/admin-api/v1/media", []byte("<?php system($_GET['c']); ?>")), 400, "UNSUPPORTED_FILE")
	r := a.upload("/admin-api/v1/media", testPNG(t))
	expect(t, r, 201, "")
	iconID := int64(r.body["id"].(float64))
	iconURL := r.body["url"].(string)
	if r.body["color"] != "#2F6BFF" || !strings.HasPrefix(iconURL, "http://img.test/") {
		t.Fatalf("图片信息不对：%v", r.body)
	}

	// 分类：slug 校验、唯一、只支持两级
	expect(t, a.post("/admin-api/v1/categories", map[string]any{"name": "电脑软件", "slug": "PC Soft"}), 400, "INVALID_SLUG")
	r = a.post("/admin-api/v1/categories", map[string]any{"name": "电脑软件", "slug": "pc", "icon": "monitor", "color": "blue"})
	expect(t, r, 201, "")
	pcID := int64(r.body["id"].(float64))
	expect(t, a.post("/admin-api/v1/categories", map[string]any{"name": "重复", "slug": "pc"}), 409, "SLUG_TAKEN")
	r = a.post("/admin-api/v1/categories", map[string]any{"name": "系统工具", "slug": "system", "parentId": pcID})
	expect(t, r, 201, "")
	sysID := int64(r.body["id"].(float64))
	expect(t, a.post("/admin-api/v1/categories", map[string]any{"name": "三级", "slug": "deep", "parentId": sysID}), 400, "INVALID_PARENT")

	// 新建草稿：正文过滤脚本，标签自动创建
	post := map[string]any{
		"type": "resource", "title": "Everything 便携版", "categoryId": sysID, "status": "draft",
		"contentHtml": `<h2>资源介绍</h2><p>按文件名秒搜<script>alert(1)</script></p><p><img src="` + iconURL + `"></p>`,
		"tags":        []string{"绿色软件", "文件搜索", "绿色软件"},
		"resource": map[string]any{
			"platform": "Windows", "version": "1.4.1", "fileSize": "1.7 MB", "iconId": iconID,
			"attrs":     []map[string]string{{"name": "兼容系统", "value": "Windows 7+"}, {"name": "", "value": ""}},
			"changelog": []map[string]string{{"version": "1.4.1", "date": "2026-10-03", "note": "修复排序"}},
		},
	}
	r = a.post("/admin-api/v1/posts", post)
	expect(t, r, 201, "")
	postID := id(r)
	if tags := r.body["tags"].([]any); len(tags) != 2 || tags[0] != "绿色软件" {
		t.Fatalf("标签应去重并保持顺序：%v", tags)
	}
	if strings.Contains(r.body["contentHtml"].(string), "script") {
		t.Fatalf("正文没有过滤脚本：%v", r.body["contentHtml"])
	}

	// 草稿在前台不可见
	expect(t, visitor.get("/api/v1/posts/"+postID), 404, "POST_NOT_FOUND")

	// 发布
	post["status"] = "published"
	r = a.put("/admin-api/v1/posts/"+postID, post)
	expect(t, r, 200, "")
	if r.body["publishedAt"] == nil {
		t.Fatalf("发布时应自动填写发布时间：%v", r.body)
	}

	r = visitor.get("/api/v1/posts/" + postID)
	expect(t, r, 200, "")
	html := r.body["contentHtml"].(string)
	if !strings.Contains(html, `src="`+iconURL+`"`) || !strings.Contains(html, `id="h-1"`) || strings.Contains(html, "@media/") {
		t.Fatalf("正文输出不对：%s", html)
	}
	res := r.body["resource"].(map[string]any)
	if res["platform"] != "Windows" || len(res["attrs"].([]any)) != 1 || res["icon"].(map[string]any)["color"] != "#2F6BFF" {
		t.Fatalf("资源信息不对：%v", res)
	}
	if r.body["category"].(map[string]any)["slug"] != "system" || r.body["author"].(map[string]any)["nickname"] != "编辑部" {
		t.Fatalf("分类或作者不对：%v", r.body)
	}
	tagSlug := r.body["tags"].([]any)[0].(map[string]any)["slug"].(string)

	// 列表筛选：父分类包含子分类、标签、平台、关键词
	for _, q := range []string{"category=pc", "category=system", "tag=" + url.QueryEscape(tagSlug), "platform=Windows", "q=" + url.QueryEscape("便携"), "type=resource"} {
		r = visitor.get("/api/v1/posts?" + q)
		expect(t, r, 200, "")
		if r.body["total"] != float64(1) {
			t.Fatalf("%s 应找到 1 篇：%v", q, r.body)
		}
	}
	expect(t, visitor.get("/api/v1/posts?type=article"), 200, "")
	expect(t, visitor.get("/api/v1/posts?category=nope"), 404, "CATEGORY_NOT_FOUND")
	expect(t, visitor.get("/api/v1/tags/nope"), 404, "TAG_NOT_FOUND")

	r = visitor.get("/api/v1/categories")
	expect(t, r, 200, "")
	root := items(t, r)[0]
	if root["slug"] != "pc" || root["postCount"] != float64(1) || len(root["children"].([]any)) != 1 {
		t.Fatalf("分类树不对：%v", root)
	}
	r = visitor.get("/api/v1/platforms?category=pc")
	if p := items(t, r); len(p) != 1 || p[0]["name"] != "Windows" {
		t.Fatalf("平台统计不对：%v", r.body)
	}
	r = visitor.get("/api/v1/tags?limit=5")
	if tags := items(t, r); len(tags) != 2 || tags[0]["postCount"] != float64(1) {
		t.Fatalf("热门标签不对：%v", r.body)
	}

	// 定时发布：发布时间未到时前台不可见
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	r = a.post("/admin-api/v1/posts", map[string]any{
		"type": "article", "title": "明天发的教程", "categoryId": pcID, "status": "published", "publishedAt": future, "contentHtml": "<p>稍后</p>",
	})
	expect(t, r, 201, "")
	expect(t, visitor.get("/api/v1/posts/"+id(r)), 404, "POST_NOT_FOUND")

	// 相关推荐：同分类的其他内容
	r = a.post("/admin-api/v1/posts", map[string]any{
		"type": "resource", "title": "Listary", "categoryId": sysID, "status": "published", "contentHtml": "<p>启动器</p>",
	})
	expect(t, r, 201, "")
	listaryID := id(r)
	// 自动截取的摘要：后台编辑时显示为空，前台展示截取结果
	if r.body["summary"] != "" {
		t.Fatalf("自动摘要在后台应显示为空：%v", r.body["summary"])
	}
	if card := visitor.get("/api/v1/posts/" + listaryID); card.body["summary"] != "启动器" {
		t.Fatalf("摘要应从正文生成：%v", card.body["summary"])
	}
	r = visitor.get("/api/v1/posts/" + postID + "/related")
	if rel := items(t, r); len(rel) != 1 || itoa(int64(rel[0]["id"].(float64))) != listaryID {
		t.Fatalf("相关推荐不对：%v", r.body)
	}

	// 分类下有内容时不能删除
	expect(t, a.del("/admin-api/v1/categories/"+itoa(sysID)), 409, "CATEGORY_IN_USE")

	// 回收站：移入后前台不可见；只能彻底删除回收站里的内容
	expect(t, a.del("/admin-api/v1/posts/"+listaryID+"?permanent=true"), 400, "NOT_IN_TRASH")
	expect(t, a.del("/admin-api/v1/posts/"+listaryID), 204, "")
	expect(t, visitor.get("/api/v1/posts/"+listaryID), 404, "POST_NOT_FOUND")
	expect(t, a.put("/admin-api/v1/posts/"+listaryID, post), 400, "POST_IN_TRASH")
	expect(t, a.post("/admin-api/v1/posts/"+listaryID+"/restore", nil), 204, "")
	r = a.get("/admin-api/v1/posts/" + listaryID)
	if r.body["status"] != "draft" {
		t.Fatalf("恢复后应为草稿：%v", r.body)
	}
	expect(t, a.del("/admin-api/v1/posts/"+listaryID), 204, "")
	expect(t, a.del("/admin-api/v1/posts/"+listaryID+"?permanent=true"), 204, "")
	expect(t, a.get("/admin-api/v1/posts/"+listaryID), 404, "POST_NOT_FOUND")

	// 后台列表默认不含回收站，可按状态筛选
	r = a.get("/admin-api/v1/posts?categoryId=" + itoa(pcID))
	if r.body["total"] != float64(2) {
		t.Fatalf("后台列表应有 2 篇：%v", r.body)
	}

	// 首页设置
	expect(t, a.put("/admin-api/v1/settings/home", map[string]any{
		"banners": []map[string]any{{"title": "x", "link": "javascript:alert(1)"}}, "notice": "",
	}), 400, "INVALID_LINK")
	expect(t, a.put("/admin-api/v1/settings/home", map[string]any{
		"banners": []map[string]any{{"title": "装机必备", "link": "/category/pc/", "imageId": iconID}}, "notice": "国庆活动",
	}), 200, "")
	r = visitor.get("/api/v1/site/home")
	expect(t, r, 200, "")
	banners := r.body["banners"].([]any)
	if len(banners) != 1 || banners[0].(map[string]any)["image"].(map[string]any)["url"] != iconURL || r.body["notice"] != "国庆活动" {
		t.Fatalf("首页设置不对：%v", r.body)
	}

	// 未登录不能调用后台接口
	expect(t, visitor.get("/admin-api/v1/posts"), 401, "UNAUTHORIZED")
}
