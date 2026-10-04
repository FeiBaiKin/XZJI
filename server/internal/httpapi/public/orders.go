package public

import (
	"context"
	"errors"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/httpapi/apierr"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/content"
	"xzji/internal/module/order"
	"xzji/internal/module/payment"
	"xzji/internal/module/wallet"
)

func (h *Handler) ListMembershipPlans(ctx context.Context, _ pubapi.ListMembershipPlansRequestObject) (pubapi.ListMembershipPlansResponseObject, error) {
	catalog, err := h.Members.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	out := pubapi.PlanCatalog{Levels: make([]pubapi.LevelPlans, len(catalog))}
	for i, lp := range catalog {
		l := lp.Level
		out.Levels[i] = pubapi.LevelPlans{
			Id: l.ID, Name: l.Name, Description: l.Description, Rank: l.Rank, DiscountRate: l.DiscountRate,
			DailyDownloadLimit: l.DailyDownloadLimit, Perks: l.Perks, Plans: make([]pubapi.MembershipPlan, len(lp.Plans)),
		}
		for j, p := range lp.Plans {
			out.Levels[i].Plans[j] = pubapi.MembershipPlan{
				Id: p.ID, LevelId: p.LevelID, Name: p.Name, DurationDays: p.DurationDays, Price: p.Price, OriginalPrice: p.OriginalPrice, Badge: p.Badge,
			}
		}
	}
	return pubapi.ListMembershipPlans200JSONResponse(out), nil
}

func toMembershipQuote(m *order.MembershipSnap) *pubapi.MembershipQuote {
	if m == nil {
		return nil
	}
	out := &pubapi.MembershipQuote{
		Kind: pubapi.MembershipQuoteKind(m.Kind), LevelName: m.LevelName, PlanName: m.PlanName, Rank: m.Rank, DurationDays: m.DurationDays,
		FromExpiresAt: m.FromExpiresAt, FromPermanent: m.FromPermanent, RemainingDays: m.RemainingDays, RemainingValue: m.RemainingValue,
		ConvertedDays: m.ConvertedDays, Deduct: m.Deduct, ExpiresAt: m.ExpiresAt,
	}
	if m.FromLevel != "" {
		from := m.FromLevel
		out.FromLevel = &from
	}
	return out
}

func toOrder(o order.Order) pubapi.Order {
	out := pubapi.Order{
		OrderNo: o.No, Type: pubapi.OrderType(o.Type), TargetId: o.TargetID, Title: o.Title,
		Amount: o.Amount, DiscountAmount: o.DiscountAmount, PayAmount: o.PayAmount, PayMethod: pubapi.OrderPayMethod(o.PayMethod),
		Status: pubapi.OrderStatus(o.Status), ExpiresAt: o.ExpiresAt, PaidAt: o.PaidAt, CreatedAt: o.CreatedAt,
		Membership: toMembershipQuote(o.Snapshot.Membership),
	}
	if g := o.Snapshot.Grant; g != nil {
		failed := g.Error != ""
		out.Grant = &pubapi.OrderGrant{Permanent: g.Permanent, ExpiresAt: g.ExpiresAt, BalanceAfter: g.BalanceAfter, Failed: &failed}
	}
	return out
}

func sceneOf(s *pubapi.PayScene) string {
	if s != nil && *s == pubapi.Mobile {
		return payment.SceneMobile
	}
	return payment.ScenePC
}

// methodStates 各支付方式是否可用；余额还要看订单类型和余额是否足够。
func (h *Handler) methodStates(ctx context.Context, scene string, t order.Type, payAmount, balance int64) ([]pubapi.PayMethodState, error) {
	states, err := h.Payment.Methods(ctx, scene)
	if err != nil {
		return nil, err
	}
	out := make([]pubapi.PayMethodState, len(states))
	for i, st := range states {
		ms := pubapi.PayMethodState{Method: pubapi.PayMethod(st.Method), Available: st.Available}
		reason := ""
		switch {
		case !st.Available && st.Method == order.MethodWechat && scene == payment.SceneMobile:
			reason = "手机浏览器暂不支持微信支付"
		case !st.Available:
			reason = "暂未开通"
		case st.Method == order.MethodBalance && t == order.TypeRecharge:
			ms.Available, reason = false, "不能用余额充值"
		case st.Method == order.MethodBalance && balance < payAmount:
			ms.Available, reason = false, "余额不足"
		}
		if reason != "" {
			ms.Reason = &reason
		}
		out[i] = ms
	}
	return out, nil
}

func orderInput(ctx context.Context, t pubapi.OrderType, targetID, amount *int64) order.Input {
	id := identity(ctx)
	return order.Input{UserID: id.UserID, Admin: id.Admin, Type: order.Type(t), TargetID: deref(targetID), Amount: deref(amount)}
}

func (h *Handler) QuoteOrder(ctx context.Context, req pubapi.QuoteOrderRequestObject) (pubapi.QuoteOrderResponseObject, error) {
	p := req.Params
	in := orderInput(ctx, p.Type, p.TargetId, p.Amount)
	q, err := h.Orders.Quote(ctx, in)
	if err != nil {
		return nil, err
	}
	balance, err := h.Wallet.Balance(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	methods, err := h.methodStates(ctx, sceneOf(p.Scene), q.Type, q.PayAmount, balance)
	if err != nil {
		return nil, err
	}
	return pubapi.QuoteOrder200JSONResponse{
		Type: pubapi.OrderType(q.Type), TargetId: q.TargetID, Title: q.Title, Amount: q.Amount, DiscountAmount: q.DiscountAmount,
		PayAmount: q.PayAmount, Membership: toMembershipQuote(q.Snapshot.Membership), Balance: balance, Methods: methods,
	}, nil
}

func toAction(a payment.Action) pubapi.PayAction {
	out := pubapi.PayAction{Kind: pubapi.PayActionKind(a.Kind), Mock: a.Mock}
	if a.QRCode != "" {
		out.QrCode = &a.QRCode
	}
	if a.URL != "" {
		out.Url = &a.URL
	}
	return out
}

func (h *Handler) CreateOrder(ctx context.Context, req pubapi.CreateOrderRequestObject) (pubapi.CreateOrderResponseObject, error) {
	b := req.Body
	in := orderInput(ctx, b.Type, b.TargetId, b.Amount)
	scene, method := sceneOf(b.Scene), string(b.PayMethod)
	// 先确认支付方式可用，避免留下一张付不了的订单
	states, err := h.Payment.Methods(ctx, scene)
	if err != nil {
		return nil, err
	}
	for _, st := range states {
		if st.Method == method && !st.Available {
			return nil, payment.ErrMethodUnavailable
		}
	}
	if method == order.MethodBalance && in.Type == order.TypeRecharge {
		return nil, order.ErrBalanceRecharge
	}
	o, err := h.Orders.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	o, act, err := h.Payment.Pay(ctx, in.UserID, o.No, method, scene, ginCtx(ctx).ClientIP())
	if err != nil {
		return nil, err
	}
	return pubapi.CreateOrder201JSONResponse{Order: toOrder(o), Action: toAction(act)}, nil
}

func (h *Handler) GetOrder(ctx context.Context, req pubapi.GetOrderRequestObject) (pubapi.GetOrderResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	o, err := h.Payment.Refresh(ctx, id.UserID, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return pubapi.GetOrder200JSONResponse(toOrder(o)), nil
}

func (h *Handler) PayOrder(ctx context.Context, req pubapi.PayOrderRequestObject) (pubapi.PayOrderResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	o, act, err := h.Payment.Pay(ctx, id.UserID, req.OrderNo, string(req.Body.PayMethod), sceneOf(req.Body.Scene), ginCtx(ctx).ClientIP())
	if err != nil {
		return nil, err
	}
	return pubapi.PayOrder200JSONResponse{Order: toOrder(o), Action: toAction(act)}, nil
}

func (h *Handler) MockPayOrder(ctx context.Context, req pubapi.MockPayOrderRequestObject) (pubapi.MockPayOrderResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	o, err := h.Payment.MockPay(ctx, id.UserID, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return pubapi.MockPayOrder200JSONResponse(toOrder(o)), nil
}

var errNotifyRejected = apierr.New(400, "NOTIFY_REJECTED", "回调验签失败")

// PayNotify 渠道回调：请求体由渠道决定，直接交给支付模块读取原始请求验签。
func (h *Handler) PayNotify(ctx context.Context, req pubapi.PayNotifyRequestObject) (pubapi.PayNotifyResponseObject, error) {
	err := h.Payment.HandleNotify(ctx, string(req.Channel), ginCtx(ctx).Request)
	if errors.Is(err, payment.ErrBadNotify) {
		h.Log.Warn("支付回调验签失败", "channel", req.Channel, "error", err)
		return nil, errNotifyRejected
	}
	if err != nil {
		return nil, err
	}
	return pubapi.PayNotify200TextResponse("success"), nil
}

func (h *Handler) GetAccountOverview(ctx context.Context, _ pubapi.GetAccountOverviewRequestObject) (pubapi.GetAccountOverviewResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	balance, err := h.Wallet.Balance(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	purchases, spent, err := h.Orders.Stats(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	rank, _, limit, err := h.Members.Active(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	used := 0
	if rank > 0 {
		if used, err = h.Content.UsedToday(ctx, id.UserID); err != nil {
			return nil, err
		}
	}
	return pubapi.GetAccountOverview200JSONResponse{
		Balance: balance, PurchaseCount: purchases, Spent: spent, TodayFreeDownloads: used, DailyLimit: limit,
	}, nil
}

func (h *Handler) ListMyOrders(ctx context.Context, req pubapi.ListMyOrdersRequestObject) (pubapi.ListMyOrdersResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	p := req.Params
	page, err := h.Orders.UserOrders(ctx, id.UserID, string(deref(p.Status)), deref(p.Page), deref(p.PageSize))
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.Order, len(page.Orders))
	for i, o := range page.Orders {
		items[i] = toOrder(o)
	}
	return pubapi.ListMyOrders200JSONResponse{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (h *Handler) ListMyPurchases(ctx context.Context, req pubapi.ListMyPurchasesRequestObject) (pubapi.ListMyPurchasesResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	page, err := h.Orders.Purchases(ctx, id.UserID, deref(req.Params.Page), deref(req.Params.PageSize))
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Purchases))
	for i, p := range page.Purchases {
		ids[i] = p.PostID
	}
	posts, err := h.Content.Cards(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.Purchase, len(page.Purchases))
	for i, p := range page.Purchases {
		items[i] = pubapi.Purchase{PostId: p.PostID, Title: p.Title, OrderNo: p.OrderNo, Amount: p.Amount, PayAmount: p.PayAmount, CreatedAt: p.CreatedAt}
		// 已删除或进了回收站的内容只显示下单时的标题
		if post, ok := posts[p.PostID]; ok && post.Status == content.StatusPublished {
			card := toCards([]*content.Post{post})[0]
			items[i].Post = &card
		}
	}
	return pubapi.ListMyPurchases200JSONResponse{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (h *Handler) ListMyLedger(ctx context.Context, req pubapi.ListMyLedgerRequestObject) (pubapi.ListMyLedgerResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	page, err := h.Wallet.Ledger(ctx, id.UserID, deref(req.Params.Page), deref(req.Params.PageSize))
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.LedgerEntry, len(page.Entries))
	for i, e := range page.Entries {
		items[i] = toLedgerEntry(e)
	}
	return pubapi.ListMyLedger200JSONResponse{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

func toLedgerEntry(e wallet.Entry) pubapi.LedgerEntry {
	return pubapi.LedgerEntry{
		Id: e.ID, Amount: e.Amount, BalanceAfter: e.BalanceAfter, BizType: pubapi.LedgerEntryBizType(e.BizType), Remark: e.Remark, CreatedAt: e.CreatedAt,
	}
}
