package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/membership"
	"xzji/internal/module/order"
	"xzji/internal/module/payment"
	"xzji/internal/module/user"
	"xzji/internal/module/wallet"
)

// ---------- 会员套餐 ----------

func toPlan(p membership.Plan) adminapi.MembershipPlan {
	return adminapi.MembershipPlan{
		Id: p.ID, LevelId: p.LevelID, Name: p.Name, DurationDays: p.DurationDays, Price: p.Price,
		OriginalPrice: p.OriginalPrice, Badge: p.Badge, Status: adminapi.LevelStatus(p.Status), Sort: p.Sort,
	}
}

func planInput(b *adminapi.PlanInput) membership.PlanInput {
	return membership.PlanInput{
		LevelID: b.LevelId, Name: b.Name, DurationDays: b.DurationDays, Price: b.Price, OriginalPrice: deref(b.OriginalPrice),
		Badge: deref(b.Badge), Status: string(deref(b.Status)), Sort: deref(b.Sort),
	}
}

func (h *Handler) ListMembershipPlans(ctx context.Context, _ adminapi.ListMembershipPlansRequestObject) (adminapi.ListMembershipPlansResponseObject, error) {
	plans, err := h.Members.Plans(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.MembershipPlan, len(plans))
	for i, p := range plans {
		items[i] = toPlan(p)
	}
	return adminapi.ListMembershipPlans200JSONResponse{Items: items}, nil
}

func (h *Handler) CreateMembershipPlan(ctx context.Context, req adminapi.CreateMembershipPlanRequestObject) (adminapi.CreateMembershipPlanResponseObject, error) {
	p, err := h.Members.CreatePlan(ctx, planInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.CreateMembershipPlan201JSONResponse(toPlan(p)), nil
}

func (h *Handler) UpdateMembershipPlan(ctx context.Context, req adminapi.UpdateMembershipPlanRequestObject) (adminapi.UpdateMembershipPlanResponseObject, error) {
	p, err := h.Members.UpdatePlan(ctx, req.Id, planInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateMembershipPlan200JSONResponse(toPlan(p)), nil
}

func (h *Handler) DeleteMembershipPlan(ctx context.Context, req adminapi.DeleteMembershipPlanRequestObject) (adminapi.DeleteMembershipPlanResponseObject, error) {
	if err := h.Members.DeletePlan(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteMembershipPlan204Response{}, nil
}

// ---------- 订单 ----------

func toUserBrief(id int64, briefs map[int64]user.Brief) adminapi.UserBrief {
	b, ok := briefs[id]
	if !ok {
		return adminapi.UserBrief{Id: id, Nickname: "已删除的用户"}
	}
	return adminapi.UserBrief{Id: b.ID, Nickname: b.Nickname, Email: b.Email}
}

func toAdminOrder(o order.Order, briefs map[int64]user.Brief) adminapi.AdminOrder {
	out := adminapi.AdminOrder{
		OrderNo: o.No, User: toUserBrief(o.UserID, briefs), Type: adminapi.OrderType(o.Type), TargetId: o.TargetID, Title: o.Title,
		Amount: o.Amount, DiscountAmount: o.DiscountAmount, PayAmount: o.PayAmount, PayMethod: adminapi.AdminOrderPayMethod(o.PayMethod),
		Status: adminapi.OrderStatus(o.Status), ExpiresAt: o.ExpiresAt, PaidAt: o.PaidAt,
		RefundMethod: adminapi.AdminOrderRefundMethod(o.RefundMethod), RefundRemark: o.RefundRemark, RefundedAt: o.RefundedAt, CreatedAt: o.CreatedAt,
	}
	if g := o.Snapshot.Grant; g != nil && g.Error != "" {
		msg := g.Error
		out.GrantError = &msg
	}
	return out
}

// adminOrder 单个订单连同下单用户。
func (h *Handler) adminOrder(ctx context.Context, o order.Order) (adminapi.AdminOrder, error) {
	briefs, err := h.Users.Briefs(ctx, []int64{o.UserID})
	if err != nil {
		return adminapi.AdminOrder{}, err
	}
	return toAdminOrder(o, briefs), nil
}

func (h *Handler) ListOrders(ctx context.Context, req adminapi.ListOrdersRequestObject) (adminapi.ListOrdersResponseObject, error) {
	p := req.Params
	f := order.AdminFilter{
		OrderNo: strings.TrimSpace(deref(p.OrderNo)), UserID: deref(p.UserId), Type: string(deref(p.Type)), Status: string(deref(p.Status)),
		PayMethod: string(deref(p.PayMethod)), Duplicate: deref(p.Duplicate), Page: deref(p.Page), PageSize: deref(p.PageSize),
	}
	if email := strings.TrimSpace(deref(p.Email)); email != "" {
		u, err := h.Users.GetByEmail(ctx, email)
		if errors.Is(err, user.ErrNotFound) {
			return adminapi.ListOrders200JSONResponse{Items: []adminapi.AdminOrder{}, Page: 1, PageSize: 20}, nil
		}
		if err != nil {
			return nil, err
		}
		f.UserID = u.ID
	}
	page, err := h.Orders.AdminList(ctx, f)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Orders))
	for i, o := range page.Orders {
		ids[i] = o.UserID
	}
	briefs, err := h.Users.Briefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminOrder, len(page.Orders))
	for i, o := range page.Orders {
		items[i] = toAdminOrder(o, briefs)
	}
	return adminapi.ListOrders200JSONResponse{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (h *Handler) GetOrder(ctx context.Context, req adminapi.GetOrderRequestObject) (adminapi.GetOrderResponseObject, error) {
	o, err := h.Orders.Get(ctx, 0, req.OrderNo)
	if err != nil {
		return nil, err
	}
	item, err := h.adminOrder(ctx, o)
	if err != nil {
		return nil, err
	}
	pays, err := h.Orders.Payments(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	snapshot := map[string]any{}
	if raw, err := json.Marshal(o.Snapshot); err == nil {
		_ = json.Unmarshal(raw, &snapshot)
	}
	out := adminapi.AdminOrderDetail{Order: item, Snapshot: snapshot, Payments: make([]adminapi.AdminPayment, len(pays))}
	for i, p := range pays {
		out.Payments[i] = adminapi.AdminPayment{
			Id: p.ID, Channel: adminapi.AdminPaymentChannel(p.Channel), TradeNo: p.TradeNo, Amount: p.Amount,
			Status: adminapi.AdminPaymentStatus(p.Status), Raw: p.Raw, CreatedAt: p.CreatedAt,
		}
	}
	return adminapi.GetOrder200JSONResponse(out), nil
}

func (h *Handler) RefundOrder(ctx context.Context, req adminapi.RefundOrderRequestObject) (adminapi.RefundOrderResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	o, err := h.Orders.Refund(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.OrderNo, string(req.Body.Method), deref(req.Body.Remark))
	if err != nil {
		return nil, err
	}
	item, err := h.adminOrder(ctx, o)
	if err != nil {
		return nil, err
	}
	return adminapi.RefundOrder200JSONResponse(item), nil
}

func (h *Handler) SyncOrder(ctx context.Context, req adminapi.SyncOrderRequestObject) (adminapi.SyncOrderResponseObject, error) {
	o, err := h.Payment.AdminSync(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	item, err := h.adminOrder(ctx, o)
	if err != nil {
		return nil, err
	}
	return adminapi.SyncOrder200JSONResponse(item), nil
}

func (h *Handler) CloseOrder(ctx context.Context, req adminapi.CloseOrderRequestObject) (adminapi.CloseOrderResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	o, err := h.Payment.AdminClose(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.OrderNo)
	if err != nil {
		return nil, err
	}
	item, err := h.adminOrder(ctx, o)
	if err != nil {
		return nil, err
	}
	return adminapi.CloseOrder200JSONResponse(item), nil
}

// ---------- 余额 ----------

func (h *Handler) ListLedger(ctx context.Context, req adminapi.ListLedgerRequestObject) (adminapi.ListLedgerResponseObject, error) {
	p := req.Params
	page, err := h.Wallet.AllLedger(ctx, wallet.LedgerFilter{
		UserID: deref(p.UserId), BizType: string(deref(p.BizType)), Page: deref(p.Page), PageSize: deref(p.PageSize),
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Entries))
	for i, e := range page.Entries {
		ids[i] = e.UserID
	}
	briefs, err := h.Users.Briefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminLedgerEntry, len(page.Entries))
	for i, e := range page.Entries {
		items[i] = adminapi.AdminLedgerEntry{
			Id: e.ID, User: toUserBrief(e.UserID, briefs), Amount: e.Amount, BalanceAfter: e.BalanceAfter,
			BizType: adminapi.LedgerBizType(e.BizType), BizId: e.BizID, Remark: e.Remark, CreatedAt: e.CreatedAt,
		}
	}
	return adminapi.ListLedger200JSONResponse{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (h *Handler) AdjustBalance(ctx context.Context, req adminapi.AdjustBalanceRequestObject) (adminapi.AdjustBalanceResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	balance, err := h.Wallet.Adjust(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.Id, req.Body.Amount, req.Body.Remark)
	if err != nil {
		return nil, err
	}
	return adminapi.AdjustBalance200JSONResponse{Balance: balance}, nil
}

// ---------- 支付设置 ----------

func (h *Handler) paymentSettings(ctx context.Context) (adminapi.PaymentSettings, error) {
	st, sec, err := h.Payment.AdminSettings(ctx)
	if err != nil {
		return adminapi.PaymentSettings{}, err
	}
	out := adminapi.PaymentSettings{
		NotifyBaseUrl: st.NotifyBaseURL, SiteUrl: st.SiteURL, Balance: st.Balance, Mock: h.Payment.Mock(),
		Alipay: adminapi.AlipaySettings{
			Enabled: st.Alipay.Enabled, Sandbox: st.Alipay.Sandbox, AppId: st.Alipay.AppID, KeyMode: adminapi.AlipaySettingsKeyMode(st.Alipay.KeyMode),
			AlipayPublicKey: st.Alipay.AlipayPublicKey, AppCert: st.Alipay.AppCert, RootCert: st.Alipay.RootCert, AlipayCert: st.Alipay.AlipayCert,
			HasPrivateKey: sec.AlipayPrivateKey,
		},
		Wechat: adminapi.WechatSettings{
			Enabled: st.Wechat.Enabled, H5: st.Wechat.H5, AppId: st.Wechat.AppID, MchId: st.Wechat.MchID, SerialNo: st.Wechat.SerialNo,
			PublicKeyId: st.Wechat.PublicKeyID, PublicKey: st.Wechat.PublicKey, HasApiV3Key: sec.WechatAPIv3Key, HasPrivateKey: sec.WechatPrivateKey,
		},
	}
	if st.NotifyBaseURL != "" {
		out.NotifyUrls.Wechat = st.NotifyBaseURL + "/api/v1/pay/notify/wechat"
		out.NotifyUrls.Alipay = st.NotifyBaseURL + "/api/v1/pay/notify/alipay"
	}
	return out, nil
}

func (h *Handler) GetPaymentSettings(ctx context.Context, _ adminapi.GetPaymentSettingsRequestObject) (adminapi.GetPaymentSettingsResponseObject, error) {
	out, err := h.paymentSettings(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.GetPaymentSettings200JSONResponse(out), nil
}

func (h *Handler) UpdatePaymentSettings(ctx context.Context, req adminapi.UpdatePaymentSettingsRequestObject) (adminapi.UpdatePaymentSettingsResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	b := req.Body
	a, w := b.Alipay, b.Wechat
	in := payment.Settings{
		NotifyBaseURL: b.NotifyBaseUrl, SiteURL: b.SiteUrl, Balance: b.Balance,
		Alipay: payment.AlipaySettings{
			Enabled: a.Enabled, Sandbox: a.Sandbox, AppID: a.AppId, PrivateKey: deref(a.PrivateKey), KeyMode: string(a.KeyMode),
			AlipayPublicKey: deref(a.AlipayPublicKey), AppCert: deref(a.AppCert), RootCert: deref(a.RootCert), AlipayCert: deref(a.AlipayCert),
		},
		Wechat: payment.WechatSettings{
			Enabled: w.Enabled, H5: w.H5, AppID: w.AppId, MchID: w.MchId, SerialNo: w.SerialNo, APIv3Key: deref(w.ApiV3Key),
			PrivateKey: deref(w.PrivateKey), PublicKeyID: deref(w.PublicKeyId), PublicKey: deref(w.PublicKey),
		},
	}
	if err := h.Payment.SaveSettings(ctx, id.UserID, ginCtx(ctx).ClientIP(), in); err != nil {
		return nil, err
	}
	out, err := h.paymentSettings(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.UpdatePaymentSettings200JSONResponse(out), nil
}
