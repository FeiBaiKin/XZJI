package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xzji/internal/config"
)

// 站点设置的保存与读取：公开接口能读到、正文经过过滤、保存后清空缓存并通知前台。
func TestSiteSettingsFlow(t *testing.T) {
	notified := make(chan http.Header, 8)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notified <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer front.Close()

	e := newEnv(t, func(c *config.Config) {
		c.Frontend = config.Frontend{PurgeURLs: []string{front.URL}, PurgeSecret: "test-secret"}
	})
	a := e.adminBrowser(t)
	visitor := e.browser(t)

	// 没设置过时返回空值，前台据此回退到站点名称、协议页提示尚未填写
	r := visitor.get("/api/v1/site/info")
	expect(t, r, 200, "")
	if r.body["footerAbout"] != "" || r.body["icp"] != "" || r.body["policeIcp"] != "" {
		t.Fatalf("未设置时应该都是空字符串：%v", r.body)
	}
	if r := visitor.get("/api/v1/site/documents"); r.body["terms"] != "" || r.body["privacy"] != "" {
		t.Fatalf("未设置时正文应该为空：%v", r.body)
	}

	// 保存设置。两份正文里带上脚本、事件属性和 javascript 链接，都会被过滤掉
	body := map[string]any{
		"footerAbout": "整理安卓、电脑软件和建站资源。",
		"icp":         "京ICP备00000000号-1",
		"policeIcp":   "京公网安备11010502000000号",
		"termsHtml": `<h2>用户协议</h2><p>使用本站即表示同意以下条款。</p>` +
			`<script>alert(1)</script><a href="javascript:alert(1)">点我</a><img src="/uploads/a.png" onerror="alert(1)">`,
		"privacyHtml": "<p>我们只收集邮箱、IP 和订单记录。</p>",
	}
	expect(t, a.put("/admin-api/v1/settings/site", body), 200, "")

	info := visitor.get("/api/v1/site/info")
	expect(t, info, 200, "")
	if info.body["icp"] != "京ICP备00000000号-1" || info.body["policeIcp"] != "京公网安备11010502000000号" {
		t.Fatalf("公开接口读到的备案信息不对：%v", info.body)
	}

	docs := visitor.get("/api/v1/site/documents")
	expect(t, docs, 200, "")
	terms, _ := docs.body["terms"].(string)
	for _, bad := range []string{"<script", "onerror", "javascript:"} {
		if strings.Contains(terms, bad) {
			t.Errorf("协议正文没有过滤掉 %q：%s", bad, terms)
		}
	}
	if !strings.Contains(terms, "使用本站即表示同意以下条款") {
		t.Errorf("正常正文被误删：%s", terms)
	}
	if privacy, _ := docs.body["privacy"].(string); !strings.Contains(privacy, "邮箱") {
		t.Errorf("隐私政策正文不对：%s", privacy)
	}

	// 保存触发清除通知，签名可校验
	select {
	case h := <-notified:
		if h.Get("X-Purge-Signature") == "" {
			t.Error("清除通知缺少签名")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("保存站点设置后应通知前台清空缓存")
	}
}

// 后台写完站点设置后，前台接口立刻返回新内容（后端缓存已被清空）。
func TestSiteSettingsPurgesPublicCache(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	visitor := e.browser(t)

	visitor.get("/api/v1/site/info")
	if r := visitor.get("/api/v1/site/info"); r.header.Get("X-Cache") != "HIT" {
		t.Fatal("重复的匿名请求应命中缓存")
	}

	expect(t, a.put("/admin-api/v1/settings/site", map[string]any{
		"footerAbout": "简介", "icp": "京ICP备00000000号-1", "policeIcp": "",
		"termsHtml": "", "privacyHtml": "",
	}), 200, "")

	r := visitor.get("/api/v1/site/info")
	if r.header.Get("X-Cache") == "HIT" {
		t.Error("保存后不该再命中旧缓存")
	}
	if r.body["icp"] != "京ICP备00000000号-1" {
		t.Errorf("清空缓存后应读到刚保存的值：%v", r.body)
	}
}
