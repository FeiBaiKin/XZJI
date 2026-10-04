package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"xzji/internal/module/order"
	"xzji/internal/module/wallet"
)

// notify 模拟支付渠道服务器的回调：不带 Cookie、Origin 和 CSRF 令牌。
func (e *env) notify(t *testing.T, channel, orderNo, tradeNo string, amount int64) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"orderNo": orderNo, "tradeNo": tradeNo, "amount": amount})
	res, err := http.Post(e.srv.URL+"/api/v1/pay/notify/"+channel, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func num(v any) int64 { return int64(v.(float64)) }

// userID 当前登录用户的 ID。
func userID(t *testing.T, b *browser) int64 {
	t.Helper()
	r := b.get("/api/v1/me")
	expect(t, r, 200, "")
	return num(r.body["id"])
}

func (e *env) balance(t *testing.T, uid int64) int64 {
	t.Helper()
	b, err := e.svc.Wallet.Balance(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// checkLedger 余额必须等于流水之和，且每条流水的 balance_after 连续。
func (e *env) checkLedger(t *testing.T, uid int64) []wallet.Entry {
	t.Helper()
	page, err := e.svc.Wallet.Ledger(context.Background(), uid, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for i := len(page.Entries) - 1; i >= 0; i-- {
		en := page.Entries[i]
		sum += en.Amount
		if en.BalanceAfter != sum {
			t.Fatalf("第 %d 条流水的变动后余额 %d 与累计 %d 不一致：%+v", i, en.BalanceAfter, sum, en)
		}
	}
	if got := e.balance(t, uid); got != sum {
		t.Fatalf("余额 %d 与流水合计 %d 不一致", got, sum)
	}
	return page.Entries
}

// paidPost 创建一篇付费资源：单独购买 price 分，会员等级 freeRank 免费。
func paidPost(t *testing.T, a *browser, catID int64, title string, price int64, freeRank int) int64 {
	t.Helper()
	r := a.post("/admin-api/v1/posts", map[string]any{
		"type": "resource", "title": title, "categoryId": catID, "status": "published",
		"contentHtml": `<p>介绍</p><div data-type="hidden-block" data-cond="paid" data-title="付费内容"><p>付费秘密</p></div>`,
		"price":       price, "memberFreeRank": freeRank, "memberDiscount": true,
		"resource": map[string]any{"downloadCond": "paid", "links": []map[string]any{{"name": "百度网盘", "url": "https://pan.baidu.com/s/x"}}},
	})
	expect(t, r, 201, "")
	return num(r.body["id"])
}

func setupLevels(t *testing.T, a *browser) (vipYear, vipMonth, svipYear int64) {
	t.Helper()
	r := a.post("/admin-api/v1/membership/levels", map[string]any{"name": "VIP", "rank": 1, "discountRate": 80, "dailyDownloadLimit": 20, "perks": []string{"会员专属标识"}})
	expect(t, r, 201, "")
	vip := num(r.body["id"])
	r = a.post("/admin-api/v1/membership/levels", map[string]any{"name": "SVIP", "rank": 2, "discountRate": 50, "dailyDownloadLimit": 0, "description": "适合经常下载"})
	expect(t, r, 201, "")
	svip := num(r.body["id"])
	plan := func(level int64, name string, days int, price int64) int64 {
		r := a.post("/admin-api/v1/membership/plans", map[string]any{"levelId": level, "name": name, "durationDays": days, "price": price})
		expect(t, r, 201, "")
		return num(r.body["id"])
	}
	vipMonth = plan(vip, "月卡", 30, 990)
	vipYear = plan(vip, "年卡", 365, 3900)
	plan(vip, "永久", 0, 9900)
	svipYear = plan(svip, "年卡", 365, 9900)
	plan(svip, "永久", 0, 19900)
	// 同一等级只能有一个启用的永久套餐；还有套餐的等级不能删除
	expect(t, a.post("/admin-api/v1/membership/plans", map[string]any{"levelId": vip, "name": "永久2", "durationDays": 0, "price": 8800}), 400, "PERMANENT_DUPLICATE")
	expect(t, a.del("/admin-api/v1/membership/levels/"+itoa(svip)), 409, "LEVEL_HAS_PLANS")
	return vipYear, vipMonth, svipYear
}

func TestTradeFlow(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	anon := e.browser(t)
	buyer := e.browser(t)
	e.register(t, buyer, "buyer@example.com", "abc12345")
	uid := userID(t, buyer)

	vipYear, vipMonth, svipYear := setupLevels(t, a)
	r := anon.get("/api/v1/membership/plans")
	expect(t, r, 200, "")
	levels := r.body["levels"].([]any)
	if len(levels) != 2 || len(levels[0].(map[string]any)["plans"].([]any)) != 3 || levels[1].(map[string]any)["description"] != "适合经常下载" {
		t.Fatalf("会员套餐目录不对：%v", r.body)
	}

	r = a.post("/admin-api/v1/categories", map[string]any{"name": "电脑软件", "slug": "pc"})
	catID := num(r.body["id"])
	pid := paidPost(t, a, catID, "付费资源", 990, 2)
	pidStr := itoa(pid)

	// 报价需要登录；余额为 0 时余额支付不可用，模拟模式下微信和支付宝可用
	expect(t, anon.get("/api/v1/orders/quote?type=post&targetId="+pidStr), 401, "UNAUTHORIZED")
	r = buyer.get("/api/v1/orders/quote?type=post&targetId=" + pidStr)
	expect(t, r, 200, "")
	if r.body["payAmount"] != float64(990) || r.body["balance"] != float64(0) {
		t.Fatalf("内容报价不对：%v", r.body)
	}
	for _, m := range r.body["methods"].([]any) {
		ms := m.(map[string]any)
		if want := ms["method"] != "balance"; ms["available"] != want {
			t.Fatalf("支付方式可用状态不对：%v", r.body["methods"])
		}
	}
	expect(t, buyer.get("/api/v1/orders/quote?type=recharge&amount=50"), 400, "INVALID_AMOUNT")
	expect(t, buyer.post("/api/v1/orders", map[string]any{"type": "recharge", "amount": 2000, "payMethod": "balance"}), 400, "BALANCE_RECHARGE")

	// ---------- 充值：回调金额不符、重复回调、重复支付 ----------
	r = buyer.post("/api/v1/orders", map[string]any{"type": "recharge", "amount": 2000, "payMethod": "wechat"})
	expect(t, r, 201, "")
	act := r.body["action"].(map[string]any)
	if act["kind"] != "qrcode" || act["mock"] != true {
		t.Fatalf("模拟渠道应返回二维码：%v", act)
	}
	recharge := r.body["order"].(map[string]any)["orderNo"].(string)

	if code := e.notify(t, "wechat", recharge, "WX-1", 1999); code == 200 {
		t.Fatal("回调金额不符时不应确认到账")
	}
	r = buyer.get("/api/v1/orders/" + recharge)
	if r.body["status"] != "pending" || e.balance(t, uid) != 0 {
		t.Fatalf("金额不符后订单应仍待支付：%v", r.body)
	}
	for range 3 {
		if code := e.notify(t, "wechat", recharge, "WX-1", 2000); code != 200 {
			t.Fatalf("回调应返回 200，得到 %d", code)
		}
	}
	r = buyer.get("/api/v1/orders/" + recharge)
	if r.body["status"] != "paid" || r.body["payMethod"] != "wechat" || e.balance(t, uid) != 2000 {
		t.Fatalf("充值到账不对：%v，余额 %d", r.body, e.balance(t, uid))
	}
	if entries := e.checkLedger(t, uid); len(entries) != 1 || entries[0].BizType != wallet.BizRecharge {
		t.Fatalf("重复回调只能入账一次：%+v", entries)
	}
	// 另一笔交易又付了同一订单：记为重复支付，不再入账
	if code := e.notify(t, "alipay", recharge, "ALI-9", 2000); code != 200 {
		t.Fatalf("重复支付的回调也应返回 200，得到 %d", code)
	}
	if e.balance(t, uid) != 2000 {
		t.Fatal("重复支付不应再次入账")
	}
	r = a.get("/admin-api/v1/orders?duplicate=true")
	expect(t, r, 200, "")
	if got := items(t, r); len(got) != 1 || got[0]["orderNo"] != recharge {
		t.Fatalf("后台应能筛出重复支付的订单：%v", r.body)
	}
	r = a.get("/admin-api/v1/orders/" + recharge)
	pays := r.body["payments"].([]any)
	if len(pays) != 2 || pays[1].(map[string]any)["status"] != "duplicate" {
		t.Fatalf("支付记录不对：%v", pays)
	}

	// ---------- 余额购买内容 ----------
	r = buyer.post("/api/v1/orders", map[string]any{"type": "post", "targetId": pid, "payMethod": "balance"})
	expect(t, r, 201, "")
	postOrder := r.body["order"].(map[string]any)
	if r.body["action"].(map[string]any)["kind"] != "paid" || postOrder["status"] != "paid" || e.balance(t, uid) != 1010 {
		t.Fatalf("余额购买不对：%v，余额 %d", r.body, e.balance(t, uid))
	}
	r = buyer.get("/api/v1/posts/" + pidStr + "/unlocked")
	if v := r.body["viewer"].(map[string]any); v["purchased"] != true || r.body["canBuy"] != false {
		t.Fatalf("购买后应已解锁：%v", r.body)
	}
	if blocks := r.body["hiddenBlocks"].([]any); blocks[0].(map[string]any)["unlocked"] != true {
		t.Fatalf("付费隐藏块应已解锁：%v", blocks)
	}
	expect(t, buyer.get("/api/v1/orders/quote?type=post&targetId="+pidStr), 409, "ALREADY_PURCHASED")
	r = buyer.get("/api/v1/me/purchases")
	if got := items(t, r); len(got) != 1 || got[0]["post"] == nil || got[0]["payAmount"] != float64(990) {
		t.Fatalf("已购列表不对：%v", r.body)
	}

	// ---------- 会员：新开通、续费、升级 ----------
	r = buyer.post("/api/v1/orders", map[string]any{"type": "membership", "targetId": vipYear, "payMethod": "alipay", "scene": "mobile"})
	expect(t, r, 201, "")
	vipOrder := r.body["order"].(map[string]any)["orderNo"].(string)
	expect(t, buyer.post("/api/v1/orders/"+vipOrder+"/mock-pay", nil), 200, "")
	r = buyer.get("/api/v1/orders/" + vipOrder)
	grant := r.body["grant"].(map[string]any)
	exp, _ := time.Parse(time.RFC3339, grant["expiresAt"].(string))
	if r.body["status"] != "paid" || time.Until(exp) < 364*24*time.Hour || time.Until(exp) > 366*24*time.Hour {
		t.Fatalf("开通 VIP 年卡不对：%v", r.body)
	}
	r = buyer.get("/api/v1/auth/session")
	if m := r.body["user"].(map[string]any)["membership"].(map[string]any); m["rank"] != float64(1) {
		t.Fatalf("开通后会员等级不对：%v", m)
	}

	r = buyer.get("/api/v1/orders/quote?type=membership&targetId=" + itoa(vipMonth))
	if m := r.body["membership"].(map[string]any); m["kind"] != "renew" {
		t.Fatalf("同等级应为续费：%v", m)
	}
	r = buyer.get("/api/v1/orders/quote?type=membership&targetId=" + itoa(svipYear))
	m := r.body["membership"].(map[string]any)
	// 剩余 364 整天 × 39 ÷ 365 = 38.89 元，÷ (99 ÷ 365) = 143 天
	if m["kind"] != "upgrade" || m["remainingDays"] != float64(364) || m["remainingValue"] != float64(3889) || m["convertedDays"] != float64(143) || r.body["payAmount"] != float64(9900) {
		t.Fatalf("升级折算不对：%v", r.body)
	}
	// 余额不足时下单失败
	expect(t, buyer.post("/api/v1/orders", map[string]any{"type": "membership", "targetId": svipYear, "payMethod": "balance"}), 409, "INSUFFICIENT_BALANCE")

	// 后台调整余额：必须写原因，写入流水
	expect(t, a.post("/admin-api/v1/users/"+itoa(uid)+"/wallet", map[string]any{"amount": 10000, "remark": ""}), 400, "REMARK_REQUIRED")
	r = a.post("/admin-api/v1/users/"+itoa(uid)+"/wallet", map[string]any{"amount": 10000, "remark": "活动赠送"})
	expect(t, r, 200, "")
	if r.body["balance"] != float64(11010) {
		t.Fatalf("调整后余额不对：%v", r.body)
	}
	expect(t, a.post("/admin-api/v1/users/"+itoa(uid)+"/wallet", map[string]any{"amount": -99999, "remark": "扣多了"}), 409, "INSUFFICIENT_BALANCE")

	r = buyer.post("/api/v1/orders", map[string]any{"type": "membership", "targetId": svipYear, "payMethod": "balance"})
	expect(t, r, 201, "")
	o := r.body["order"].(map[string]any)
	exp, _ = time.Parse(time.RFC3339, o["grant"].(map[string]any)["expiresAt"].(string))
	if o["title"] != "SVIP 年卡（升级）" || time.Until(exp) < (365+142)*24*time.Hour || time.Until(exp) > (365+144)*24*time.Hour {
		t.Fatalf("升级 SVIP 不对：%v", o)
	}
	expect(t, buyer.get("/api/v1/orders/quote?type=membership&targetId="+itoa(vipYear)), 400, "PLAN_DOWNGRADE")
	// SVIP 可以免费查看，不需要再买
	pid2 := paidPost(t, a, catID, "另一个付费资源", 500, 2)
	expect(t, buyer.get("/api/v1/orders/quote?type=post&targetId="+itoa(pid2)), 409, "MEMBER_FREE")

	// ---------- 概览与列表 ----------
	r = buyer.get("/api/v1/me/overview")
	if r.body["balance"] != float64(1110) || r.body["purchaseCount"] != float64(1) || r.body["spent"] != float64(990+3900+9900) {
		t.Fatalf("概览不对：%v", r.body)
	}
	r = buyer.get("/api/v1/me/orders?status=paid")
	if got := items(t, r); len(got) != 4 {
		t.Fatalf("已支付订单数不对：%v", r.body)
	}
	r = buyer.get("/api/v1/me/wallet/ledger")
	if got := items(t, r); len(got) != 4 || got[0]["bizType"] != "purchase" || got[1]["bizType"] != "admin_adjust" {
		t.Fatalf("余额明细不对：%v", r.body)
	}

	// ---------- 退款 ----------
	postNo := postOrder["orderNo"].(string)
	expect(t, a.post("/admin-api/v1/orders/"+postNo+"/refund", map[string]any{"method": "original"}), 400, "INVALID_REFUND_METHOD")
	r = a.post("/admin-api/v1/orders/"+postNo+"/refund", map[string]any{"method": "balance", "remark": "用户误购"})
	expect(t, r, 200, "")
	if r.body["status"] != "refunded" || e.balance(t, uid) != 1110+990 {
		t.Fatalf("退款到余额不对：%v，余额 %d", r.body, e.balance(t, uid))
	}
	expect(t, a.post("/admin-api/v1/orders/"+postNo+"/refund", map[string]any{"method": "balance"}), 409, "NOT_REFUNDABLE")
	if ok, _ := e.svc.Orders.HasPurchased(context.Background(), uid, pid); ok {
		t.Fatal("退款后应收回已购内容")
	}
	expect(t, a.post("/admin-api/v1/orders/"+recharge+"/refund", map[string]any{"method": "balance"}), 400, "INVALID_REFUND_METHOD")
	r = a.post("/admin-api/v1/orders/"+recharge+"/refund", map[string]any{"method": "original"})
	expect(t, r, 200, "")
	if e.balance(t, uid) != 1110+990-2000 {
		t.Fatalf("充值原路退回后应扣回余额，余额 %d", e.balance(t, uid))
	}
	e.checkLedger(t, uid)

	r = a.get("/admin-api/v1/ledger?userId=" + itoa(uid) + "&bizType=refund")
	if got := items(t, r); len(got) != 2 || got[0]["user"].(map[string]any)["email"] != "buyer@example.com" {
		t.Fatalf("后台流水筛选不对：%v", r.body)
	}
	r = a.get("/admin-api/v1/users?keyword=buyer")
	if got := items(t, r); len(got) != 1 || got[0]["balance"] != float64(100) {
		t.Fatalf("用户列表的余额不对：%v", r.body)
	}
}

func TestOrderTimeoutAndLateNotify(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	buyer := e.browser(t)
	e.register(t, buyer, "late@example.com", "abc12345")
	uid := userID(t, buyer)
	r := a.post("/admin-api/v1/categories", map[string]any{"name": "软件", "slug": "soft"})
	catID := num(r.body["id"])
	pid := paidPost(t, a, catID, "付费资源", 300, 0)
	ctx := context.Background()

	r = buyer.post("/api/v1/orders", map[string]any{"type": "post", "targetId": pid, "payMethod": "wechat"})
	expect(t, r, 201, "")
	no := r.body["order"].(map[string]any)["orderNo"].(string)
	// 另一张订单用来测试过期后不能用余额支付
	o2, err := e.svc.Orders.Create(ctx, order.Input{UserID: uid, Type: order.TypePost, TargetID: pid})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Wallet.Adjust(ctx, 1, "", uid, 1000, "测试"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.ExecContext(ctx, "UPDATE orders SET expires_at = ? WHERE user_id = ?", time.Now().UTC().Add(-time.Minute), uid); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Orders.PayWithBalance(ctx, uid, o2.No); !errors.Is(err, order.ErrExpired) {
		t.Fatalf("过期订单不能用余额支付，得到 %v", err)
	}

	if err := e.svc.Payment.CloseExpired(ctx); err != nil {
		t.Fatal(err)
	}
	r = buyer.get("/api/v1/orders/" + no)
	if r.body["status"] != "closed" {
		t.Fatalf("超时订单应已关闭：%v", r.body)
	}
	expect(t, buyer.post("/api/v1/orders/"+no+"/pay", map[string]any{"payMethod": "alipay"}), 409, "ORDER_NOT_PENDING")

	// 关闭后才收到的回调：钱已收到，订单改为已支付并发放权益
	if code := e.notify(t, "wechat", no, "WX-LATE", 300); code != 200 {
		t.Fatalf("迟到的回调应返回 200，得到 %d", code)
	}
	r = buyer.get("/api/v1/orders/" + no)
	if r.body["status"] != "paid" {
		t.Fatalf("迟到的回调应确认支付：%v", r.body)
	}
	if ok, _ := e.svc.Orders.HasPurchased(ctx, uid, pid); !ok {
		t.Fatal("迟到的回调也应发放权益")
	}

	// 补单：发起了支付但回调丢失，补单任务查询渠道后确认
	pid3 := paidPost(t, a, catID, "第三个资源", 200, 0)
	r = buyer.post("/api/v1/orders", map[string]any{"type": "post", "targetId": pid3, "payMethod": "alipay"})
	no3 := r.body["order"].(map[string]any)["orderNo"].(string)
	e.svc.Payment.MockChannelPaid(order.MethodAlipay, no3, 200)
	if err := e.svc.Payment.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if ok, _ := e.svc.Orders.HasPurchased(ctx, uid, pid3); !ok {
		t.Fatal("补单任务应确认到账并发放权益")
	}
}

func TestConcurrentBalancePayments(t *testing.T) {
	e := newEnv(t)
	a := e.adminBrowser(t)
	buyer := e.browser(t)
	e.register(t, buyer, "rush@example.com", "abc12345")
	uid := userID(t, buyer)
	ctx := context.Background()
	r := a.post("/admin-api/v1/categories", map[string]any{"name": "软件", "slug": "soft"})
	catID := num(r.body["id"])
	if _, err := e.svc.Wallet.Adjust(ctx, 1, "", uid, 1000, "测试"); err != nil {
		t.Fatal(err)
	}

	// 5 篇各 300 分的内容同时用余额支付，余额 1000 只够买 3 篇
	var orders []string
	for i := range 5 {
		pid := paidPost(t, a, catID, "资源"+itoa(int64(i)), 300, 0)
		o, err := e.svc.Orders.Create(ctx, order.Input{UserID: uid, Type: order.TypePost, TargetID: pid})
		if err != nil {
			t.Fatal(err)
		}
		orders = append(orders, o.No)
	}
	// 同一篇内容的 3 张订单同时支付，只能扣一次钱
	same := paidPost(t, a, catID, "同一篇", 50, 0)
	for range 3 {
		o, err := e.svc.Orders.Create(ctx, order.Input{UserID: uid, Type: order.TypePost, TargetID: same})
		if err != nil {
			t.Fatal(err)
		}
		orders = append(orders, o.No)
	}

	var wg sync.WaitGroup
	errs := make([]error, len(orders))
	for i, no := range orders {
		wg.Go(func() {
			_, errs[i] = e.svc.Orders.PayWithBalance(ctx, uid, no)
		})
	}
	wg.Wait()

	var paid, insufficient, repurchase int
	for _, err := range errs {
		switch {
		case err == nil:
			paid++
		case errors.Is(err, wallet.ErrInsufficient):
			insufficient++
		case errors.Is(err, order.ErrAlreadyBought):
			repurchase++
		default:
			t.Fatalf("意外的错误：%v", err)
		}
	}
	balance := e.balance(t, uid)
	if balance < 0 || repurchase != 2 || paid+insufficient != 6 {
		t.Fatalf("并发扣款结果不对：成功 %d、余额不足 %d、重复购买 %d，余额 %d", paid, insufficient, repurchase, balance)
	}
	// 成功的笔数和余额必须对得上：1000 - 300×n - 50（如果同一篇买成功了）
	entries := e.checkLedger(t, uid)
	var spent int64
	for _, en := range entries {
		if en.BizType == wallet.BizPurchase {
			spent -= en.Amount
		}
	}
	if int(len(entries)-1) != paid || balance != 1000-spent {
		t.Fatalf("流水笔数 %d 与成功笔数 %d 不一致，余额 %d，支出 %d", len(entries)-1, paid, balance, spent)
	}
}
