package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// unlockReasons 把解锁情况整理成 序号→原因（已解锁为 ok），方便断言。
func unlockReasons(t *testing.T, r resp) map[int]string {
	t.Helper()
	out := map[int]string{}
	for _, raw := range r.body["hiddenBlocks"].([]any) {
		b := raw.(map[string]any)
		seq := int(b["seq"].(float64))
		if b["unlocked"].(bool) {
			out[seq] = "ok"
			continue
		}
		out[seq] = b["lock"].(map[string]any)["reason"].(string)
	}
	return out
}

func sameReasons(a, b map[int]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestAccessControlFlow(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	anon := e.browser(t)
	reader := e.browser(t)
	e.register(t, reader, "reader@example.com", "abc12345")
	vip := e.browser(t)
	e.register(t, vip, "vip@example.com", "abc12345")

	// 会员等级：VIP 8 折、每天 1 次会员免费下载；SVIP 不限次数
	r := a.post("/admin-api/v1/membership/levels", map[string]any{"name": "VIP", "rank": 1, "discountRate": 80, "dailyDownloadLimit": 1})
	expect(t, r, 201, "")
	vipLevel := id(r)
	expect(t, a.post("/admin-api/v1/membership/levels", map[string]any{"name": "SVIP", "rank": 2, "discountRate": 70, "dailyDownloadLimit": 0}), 201, "")
	expect(t, a.post("/admin-api/v1/membership/levels", map[string]any{"name": "重复", "rank": 1, "discountRate": 100, "dailyDownloadLimit": 0}), 409, "RANK_TAKEN")

	r = a.post("/admin-api/v1/categories", map[string]any{"name": "电脑软件", "slug": "pc"})
	catID := int64(r.body["id"].(float64))

	content := `<p>公开介绍</p>` +
		`<div data-type="hidden-block" data-cond="login" data-title="登录内容"><p>登录秘密</p></div>` +
		`<div data-type="hidden-block" data-cond="comment" data-title=""><p>评论秘密</p></div>` +
		`<div data-type="hidden-block" data-cond="paid" data-title="付费内容"><p>付费秘密</p></div>` +
		`<div data-type="hidden-block" data-cond="member:1" data-title=""><p>会员秘密</p></div>`
	post := map[string]any{
		"type": "resource", "title": "Everything 便携版", "categoryId": catID, "status": "published", "contentHtml": content,
		"price": 990, "memberFreeRank": 2, "memberDiscount": true,
		"resource": map[string]any{
			"downloadCond": "member:1",
			"links": []map[string]any{
				{"name": "百度网盘", "url": "https://pan.baidu.com/s/secret-a", "extractCode": "ab12", "unzipPassword": "xzji"},
				{"name": "夸克网盘", "url": "https://pan.quark.cn/s/secret-b"},
			},
		},
	}

	// 保存时的校验
	bad := map[string]any{}
	for k, v := range post {
		bad[k] = v
	}
	bad["price"], bad["memberFreeRank"] = 0, 0
	expect(t, a.post("/admin-api/v1/posts", bad), 400, "PAID_NO_PRICE")
	bad["price"], bad["contentHtml"] = 990, `<ul><li><div data-type="hidden-block" data-cond="login"><p>x</p></div></li></ul>`
	expect(t, a.post("/admin-api/v1/posts", bad), 400, "NESTED_HIDDEN")
	bad["contentHtml"] = `<div data-type="hidden-block" data-cond="vip"><p>x</p></div>`
	expect(t, a.post("/admin-api/v1/posts", bad), 400, "INVALID_COND")
	bad["contentHtml"], bad["resource"] = content, map[string]any{"links": []map[string]any{{"name": "坏链接", "url": "javascript:alert(1)"}}}
	expect(t, a.post("/admin-api/v1/posts", bad), 400, "INVALID_LINK")

	r = a.post("/admin-api/v1/posts", post)
	expect(t, r, 201, "")
	pid := id(r)
	links := r.body["resource"].(map[string]any)["links"].([]any)
	linkID := itoa(int64(links[0].(map[string]any)["id"].(float64)))
	// 后台编辑时隐藏块还原回正文
	if !strings.Contains(r.body["contentHtml"].(string), `<div data-type="hidden-block" data-cond="paid" data-title="付费内容"><p>付费秘密</p></div>`) {
		t.Fatalf("后台正文没有还原隐藏块：%s", r.body["contentHtml"])
	}

	// 前台详情：只有公开内容和占位符，下载地址只返回名称
	r = anon.get("/api/v1/posts/" + pid)
	expect(t, r, 200, "")
	raw, _ := json.Marshal(r.body)
	for _, secret := range []string{"登录秘密", "评论秘密", "付费秘密", "会员秘密", "secret-a", "ab12", "xzji\""} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("前台详情泄露了受保护的内容 %q：%s", secret, raw)
		}
	}
	if !strings.Contains(r.body["contentHtml"].(string), `<div data-hidden-block="3"></div>`) || len(r.body["hiddenBlocks"].([]any)) != 4 {
		t.Fatalf("占位符或隐藏块信息不对：%v", r.body)
	}
	if r.body["price"] != float64(990) || r.body["resource"].(map[string]any)["downloadCond"] != "member:1" {
		t.Fatalf("付费信息不对：%v", r.body)
	}

	// 未登录：全部锁定，下载要求登录
	r = anon.get("/api/v1/posts/" + pid + "/unlocked")
	expect(t, r, 200, "")
	if got := unlockReasons(t, r); !sameReasons(got, map[int]string{1: "login", 2: "comment", 3: "paid", 4: "member"}) {
		t.Fatalf("未登录的解锁情况不对：%v", got)
	}
	if r.body["price"] != float64(990) || r.body["canBuy"] != true {
		t.Fatalf("未登录的价格不对：%v", r.body)
	}
	expect(t, anon.post("/api/v1/posts/"+pid+"/links/"+linkID+"/download", nil), 401, "LOGIN_REQUIRED")

	// 普通用户：登录内容可见；评论审核通过后评论内容可见；会员下载被拒绝
	r = reader.get("/api/v1/posts/" + pid + "/unlocked")
	if got := unlockReasons(t, r); !sameReasons(got, map[int]string{1: "ok", 2: "comment", 3: "paid", 4: "member"}) {
		t.Fatalf("普通用户的解锁情况不对：%v", got)
	}
	expect(t, reader.post("/api/v1/posts/"+pid+"/links/"+linkID+"/download", nil), 403, "DOWNLOAD_LOCKED")
	expect(t, anon.post("/api/v1/posts/"+pid+"/comments", map[string]any{"content": "谢谢分享"}), 401, "UNAUTHORIZED")
	r = reader.post("/api/v1/posts/"+pid+"/comments", map[string]any{"content": "谢谢分享，很好用"})
	expect(t, r, 201, "")
	commentID := id(r)
	if r.body["status"] != "pending" {
		t.Fatalf("默认应先审核：%v", r.body)
	}
	expect(t, reader.post("/api/v1/posts/"+pid+"/comments", map[string]any{"content": "再来一条"}), 429, "TOO_MANY_REQUESTS")
	// 待审核的评论只有作者自己看得到
	if len(items(t, anon.get("/api/v1/posts/"+pid+"/comments"))) != 0 {
		t.Fatal("访客不应看到待审核的评论")
	}
	if len(items(t, reader.get("/api/v1/posts/"+pid+"/comments"))) != 1 {
		t.Fatal("作者应能看到自己待审核的评论")
	}
	if unlockReasons(t, reader.get("/api/v1/posts/" + pid + "/unlocked"))[2] != "comment" {
		t.Fatal("评论审核前不应解锁")
	}
	r = a.get("/admin-api/v1/comments?status=pending")
	if r.body["total"] != float64(1) || items(t, r)[0]["postTitle"] != "Everything 便携版" {
		t.Fatalf("后台待审核评论不对：%v", r.body)
	}
	expect(t, a.do(http.MethodPatch, "/admin-api/v1/comments/"+commentID, map[string]any{"status": "approved"}, true, nil), 200, "")
	if unlockReasons(t, reader.get("/api/v1/posts/" + pid + "/unlocked"))[2] != "ok" {
		t.Fatal("评论审核通过后应解锁")
	}
	if anon.get("/api/v1/posts/" + pid).body["commentCount"] != float64(1) {
		t.Fatal("评论数没有更新")
	}

	// 开通 VIP：会员内容可见；付费内容仍需 SVIP 或购买，价格打 8 折；下载每天 1 次
	expect(t, a.put("/admin-api/v1/users/999/membership", map[string]any{"levelId": json.Number(vipLevel)}), 404, "USER_NOT_FOUND")
	r = a.get("/admin-api/v1/users?keyword=vip@")
	vipID := itoa(int64(items(t, r)[0]["id"].(float64)))
	expect(t, a.put("/admin-api/v1/users/"+vipID+"/membership", map[string]any{"levelId": json.Number(vipLevel), "expiresAt": "2020-01-01T00:00:00Z"}), 400, "INVALID_EXPIRES")
	r = a.put("/admin-api/v1/users/"+vipID+"/membership", map[string]any{"levelId": json.Number(vipLevel)})
	expect(t, r, 200, "")
	if m := r.body["membership"].(map[string]any); m["name"] != "VIP" || m["active"] != true || m["expiresAt"] != nil {
		t.Fatalf("会员状态不对：%v", r.body)
	}
	if vip.get("/api/v1/auth/session").body["user"].(map[string]any)["membership"].(map[string]any)["rank"] != float64(1) {
		t.Fatal("登录状态里应带会员等级")
	}
	expect(t, a.del("/admin-api/v1/membership/levels/"+vipLevel), 409, "LEVEL_IN_USE")

	r = vip.get("/api/v1/posts/" + pid + "/unlocked")
	if got := unlockReasons(t, r); !sameReasons(got, map[int]string{1: "ok", 2: "comment", 3: "paid", 4: "ok"}) {
		t.Fatalf("VIP 的解锁情况不对：%v", got)
	}
	if r.body["price"] != float64(792) || r.body["download"].(map[string]any)["memberFree"] != true {
		t.Fatalf("VIP 的价格或下载情况不对：%v", r.body)
	}
	r = vip.post("/api/v1/posts/"+pid+"/links/"+linkID+"/download", nil)
	expect(t, r, 200, "")
	if r.body["url"] != "https://pan.baidu.com/s/secret-a" || r.body["extractCode"] != "ab12" || r.body["unzipPassword"] != "xzji" {
		t.Fatalf("下载地址不对：%v", r.body)
	}
	// 同一篇当天再次获取不重复计数
	expect(t, vip.post("/api/v1/posts/"+pid+"/links/"+linkID+"/download", nil), 200, "")
	if vip.get("/api/v1/posts/"+pid+"/unlocked").body["download"].(map[string]any)["usedToday"] != float64(1) {
		t.Fatal("今日已用次数应为 1")
	}
	r = a.post("/admin-api/v1/posts", map[string]any{
		"type": "resource", "title": "Snipaste", "categoryId": catID, "status": "published", "contentHtml": "<p>截图</p>",
		"resource": map[string]any{"downloadCond": "member:1", "links": []map[string]any{{"name": "蓝奏云", "url": "https://lanzou.com/x"}}},
	})
	pid2 := id(r)
	link2 := itoa(int64(r.body["resource"].(map[string]any)["links"].([]any)[0].(map[string]any)["id"].(float64)))
	expect(t, vip.post("/api/v1/posts/"+pid2+"/links/"+link2+"/download", nil), 429, "DAILY_LIMIT")

	// 管理员在前台登录后始终可见
	adminFront := e.browser(t)
	expect(t, adminFront.post("/api/v1/auth/login", map[string]any{"email": "editor@example.com", "password": "edit12345"}), 200, "")
	if got := unlockReasons(t, adminFront.get("/api/v1/posts/"+pid+"/unlocked")); !sameReasons(got, map[int]string{1: "ok", 2: "ok", 3: "ok", 4: "ok"}) {
		t.Fatalf("管理员应全部可见：%v", got)
	}

	// 价格筛选
	expect(t, a.post("/admin-api/v1/posts", map[string]any{
		"type": "resource", "title": "免费工具", "categoryId": catID, "status": "published", "contentHtml": "<p>免费</p>",
	}), 201, "")
	for q, want := range map[string]float64{"vip": 2, "free": 1, "paid": 0, "login": 0} {
		if n := anon.get("/api/v1/posts?access=" + q).body["total"]; n != want {
			t.Errorf("access=%s 应有 %v 篇，得到 %v", q, want, n)
		}
	}

	// 回复：不审核模式下直接显示；回复楼中楼时记录被回复的人
	expect(t, a.put("/admin-api/v1/settings/comment", map[string]any{"moderation": "none"}), 200, "")
	r = vip.post("/api/v1/posts/"+pid+"/comments", map[string]any{"content": "我也在用", "replyToId": json.Number(commentID)})
	expect(t, r, 201, "")
	if r.body["status"] != "approved" {
		t.Fatalf("不审核模式应直接通过：%v", r.body)
	}
	replyID := id(r)
	readerID := itoa(int64(items(t, a.get("/admin-api/v1/users?keyword=reader@"))[0]["id"].(float64)))
	e.svc.Limiter.Reset("comment:interval:" + readerID)
	r = reader.post("/api/v1/posts/"+pid+"/comments", map[string]any{"content": "好的，谢谢", "replyToId": json.Number(replyID)})
	expect(t, r, 201, "")
	if r.body["replyTo"].(map[string]any)["nickname"] != "vip" {
		t.Fatalf("楼中楼应记录被回复的人：%v", r.body)
	}
	root := items(t, anon.get("/api/v1/posts/"+pid+"/comments"))
	if len(root) != 1 || len(root[0]["replies"].([]any)) != 2 {
		t.Fatalf("回复应挂在一级评论下：%v", root)
	}
	if root[0]["replies"].([]any)[0].(map[string]any)["author"].(map[string]any)["badge"] != "VIP" {
		t.Fatalf("评论区应显示会员标识：%v", root)
	}
	if anon.get("/api/v1/posts/" + pid).body["commentCount"] != float64(3) {
		t.Fatal("评论数应为 3")
	}

	// 失效反馈
	expect(t, reader.post("/api/v1/posts/"+pid+"/links/"+linkID+"/reports", map[string]any{"reason": "打不开"}), 204, "")
	expect(t, reader.post("/api/v1/posts/"+pid+"/links/"+linkID+"/reports", map[string]any{}), 409, "ALREADY_REPORTED")
	r = a.get("/admin-api/v1/link-reports?status=pending")
	reports := items(t, r)
	if len(reports) != 1 || reports[0]["linkName"] != "百度网盘" || reports[0]["user"].(map[string]any)["nickname"] != "reader" {
		t.Fatalf("失效反馈不对：%v", r.body)
	}
	expect(t, a.do(http.MethodPatch, "/admin-api/v1/link-reports/"+itoa(int64(reports[0]["id"].(float64))), map[string]any{"status": "resolved"}, true, nil), 204, "")
	if a.get("/admin-api/v1/link-reports?status=pending").body["total"] != float64(0) {
		t.Fatal("处理后不应再是待处理")
	}

	// 删除一级评论时连同回复一起删除
	expect(t, a.del("/admin-api/v1/comments/"+commentID), 204, "")
	if anon.get("/api/v1/posts/" + pid).body["commentCount"] != float64(0) {
		t.Fatal("删除后评论数应为 0")
	}

	// 取消会员后会员内容重新锁定
	expect(t, a.del("/admin-api/v1/users/"+vipID+"/membership"), 204, "")
	if unlockReasons(t, vip.get("/api/v1/posts/" + pid + "/unlocked"))[4] != "member" {
		t.Fatal("取消会员后会员内容应锁定")
	}
}
