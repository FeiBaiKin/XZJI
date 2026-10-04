// Package payment 对接第三方支付渠道：发起支付、接收回调、主动查询补单、关闭超时订单。
// 本模块没有自己的表，订单状态和支付记录都通过 order.Service 读写。
package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"xzji/internal/module/order"
	"xzji/internal/module/setting"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/ratelimit"
	"xzji/internal/platform/secret"
)

const (
	// 渠道客户端缓存时间；微信平台证书在重建时重新下载
	gatewayTTL = 12 * time.Hour
	// 补单只查最近 2 小时内发起的订单，更早的由超时关闭任务兜底
	reconcileWindow = 2 * time.Hour
	// 前台轮询订单状态时，同一订单每 3 秒最多主动查询一次渠道
	syncInterval = 3 * time.Second
	// notifyPath 回调地址的路径前缀，与前台接口的 /api/v1 一致
	notifyPath = "/api/v1/pay/notify/"
)

var (
	ErrMethodUnavailable = errs.Invalid("PAY_METHOD_UNAVAILABLE", "这种支付方式暂不可用，请换一种")
	ErrChannel           = errs.New(errs.KindUnavailable, "PAY_CHANNEL_ERROR", "支付渠道暂时不可用，请换一种方式或稍后再试")
	ErrNotStarted        = errs.Invalid("PAY_NOT_STARTED", "请先选择微信或支付宝发起支付")
	ErrMockDisabled      = errs.NotFound("NOT_FOUND", "请求的地址不存在")
	ErrUnknownChannel    = errs.NotFound("NOT_FOUND", "请求的地址不存在")
)

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

type gatewaySet struct {
	st    Settings
	gws   map[string]Gateway
	errs  map[string]error
	built time.Time
}

type Service struct {
	settings *setting.Service
	box      *secret.Box
	orders   *order.Service
	audit    Auditor
	limiter  ratelimit.Limiter
	log      *slog.Logger
	// mocks 不为 nil 时微信、支付宝都走模拟渠道
	mocks map[string]*mockGateway
	now   func() time.Time

	mu    sync.Mutex
	cache *gatewaySet
}

// NewService mock 为 true 时启用模拟渠道，只能用于开发环境（配置加载时已校验）。
func NewService(settings *setting.Service, box *secret.Box, orders *order.Service, audit Auditor, limiter ratelimit.Limiter, log *slog.Logger, mock bool) *Service {
	s := &Service{settings: settings, box: box, orders: orders, audit: audit, limiter: limiter, log: log, now: time.Now}
	if mock {
		s.mocks = map[string]*mockGateway{order.MethodWechat: newMock(order.MethodWechat), order.MethodAlipay: newMock(order.MethodAlipay)}
	}
	return s
}

// Mock 是否启用了模拟渠道。
func (s *Service) Mock() bool { return s.mocks != nil }

func (s *Service) reset() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

func (s *Service) notifyURL(st Settings, channel string) string {
	return st.NotifyBaseURL + notifyPath + channel
}

// gateways 按支付设置创建渠道客户端。填写了参数的渠道都会创建（包括停用的），以便继续处理停用前订单的回调和补单。
func (s *Service) gateways(ctx context.Context) (*gatewaySet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache != nil && s.now().Sub(s.cache.built) < gatewayTTL {
		return s.cache, nil
	}
	st, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	set := &gatewaySet{st: st, gws: map[string]Gateway{}, errs: map[string]error{}, built: s.now()}
	if s.mocks != nil {
		for ch, g := range s.mocks {
			set.gws[ch] = g
		}
	} else {
		if st.Alipay.AppID != "" {
			if g, err := newAlipay(st.Alipay, s.notifyURL(st, order.MethodAlipay)); err != nil {
				set.errs[order.MethodAlipay] = err
			} else {
				set.gws[order.MethodAlipay] = g
			}
		}
		if st.Wechat.MchID != "" {
			if g, err := newWechat(ctx, st.Wechat, s.notifyURL(st, order.MethodWechat)); err != nil {
				set.errs[order.MethodWechat] = err
			} else {
				set.gws[order.MethodWechat] = g
			}
		}
		for ch, err := range set.errs {
			s.log.Error("支付渠道初始化失败", "channel", ch, "error", err)
		}
	}
	s.cache = set
	return set, nil
}

// usable 渠道是否可以发起新的支付。
func (set *gatewaySet) usable(mock bool, method, scene string) (Gateway, bool) {
	g, ok := set.gws[method]
	if !ok || !g.Supports(scene) {
		return nil, false
	}
	if mock {
		return g, true
	}
	switch method {
	case order.MethodAlipay:
		return g, set.st.Alipay.Enabled
	case order.MethodWechat:
		return g, set.st.Wechat.Enabled
	}
	return nil, false
}

type MethodState struct {
	Method    string
	Available bool
}

// Methods 各支付方式在该场景下是否可用。余额是否足够由调用方结合订单判断。
func (s *Service) Methods(ctx context.Context, scene string) ([]MethodState, error) {
	set, err := s.gateways(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]MethodState, 0, 3)
	for _, m := range []string{order.MethodWechat, order.MethodAlipay} {
		_, ok := set.usable(s.Mock(), m, scene)
		out = append(out, MethodState{Method: m, Available: ok})
	}
	return append(out, MethodState{Method: order.MethodBalance, Available: set.st.Balance}), nil
}

func thirdParty(method string) bool {
	return method == order.MethodWechat || method == order.MethodAlipay
}

// Pay 为待支付订单发起支付。余额支付直接完成；第三方支付返回二维码或跳转地址。
// 换了渠道时先关闭原渠道的交易，避免两边都付款。
func (s *Service) Pay(ctx context.Context, userID int64, orderNo, method, scene, clientIP string) (order.Order, Action, error) {
	o, err := s.orders.Get(ctx, userID, orderNo)
	if err != nil {
		return o, Action{}, err
	}
	if o.Status != order.StatusPending {
		return o, Action{}, order.ErrNotPending
	}
	set, err := s.gateways(ctx)
	if err != nil {
		return o, Action{}, err
	}
	if method == order.MethodBalance {
		if !set.st.Balance {
			return o, Action{}, ErrMethodUnavailable
		}
		o, err = s.orders.PayWithBalance(ctx, userID, orderNo)
		return o, Action{Kind: ActionPaid}, err
	}
	if scene != SceneMobile {
		scene = ScenePC
	}
	g, ok := set.usable(s.Mock(), method, scene)
	if !ok {
		return o, Action{}, ErrMethodUnavailable
	}
	// 二维码至少留 1 分钟给用户扫
	if o.ExpiresAt.Before(s.now().Add(time.Minute)) {
		return o, Action{}, order.ErrExpired
	}
	if thirdParty(o.PayMethod) && o.PayMethod != method {
		if old, ok := set.gws[o.PayMethod]; ok {
			if err := old.Close(ctx, o.No); err != nil {
				s.log.Warn("切换支付方式时关闭原渠道交易失败", "order_no", o.No, "channel", o.PayMethod, "error", err)
			}
		}
	}
	// 先记下支付方式再请求渠道：即使请求后进程退出，补单任务也知道去哪个渠道查询
	if err := s.orders.StartPay(ctx, o.ID, method); err != nil {
		return o, Action{}, err
	}
	act, err := g.Create(ctx, PayRequest{
		OrderNo: o.No, Title: o.Title, Amount: o.PayAmount, ExpiresAt: o.ExpiresAt, Scene: scene, ClientIP: clientIP,
		NotifyURL: s.notifyURL(set.st, method), ReturnURL: set.st.SiteURL + "/pay/result?no=" + o.No,
	})
	if err != nil {
		s.log.Error("发起支付失败", "order_no", o.No, "channel", method, "error", err)
		return o, Action{}, ErrChannel
	}
	o.PayMethod = method
	return o, act, nil
}

// HandleNotify 处理渠道的异步回调。返回 nil 表示可以告诉渠道已收到；返回错误时渠道会稍后重试。
func (s *Service) HandleNotify(ctx context.Context, channel string, r *http.Request) error {
	if !thirdParty(channel) {
		return ErrUnknownChannel
	}
	set, err := s.gateways(ctx)
	if err != nil {
		return err
	}
	g, ok := set.gws[channel]
	if !ok {
		return fmt.Errorf("收到 %s 回调，但没有配置该渠道", channel)
	}
	orderNo, paid, err := g.Notify(r)
	if err != nil {
		return err
	}
	if paid == nil {
		return nil
	}
	return s.confirm(ctx, channel, orderNo, paid)
}

func (s *Service) confirm(ctx context.Context, channel, orderNo string, paid *Paid) error {
	out, err := s.orders.ConfirmPaid(ctx, order.Notice{OrderNo: orderNo, Channel: channel, TradeNo: paid.TradeNo, Amount: paid.Amount, Raw: paid.Raw})
	switch {
	case errors.Is(err, order.ErrAmountMismatch):
		s.log.Error("支付金额与订单不符，未确认到账", "order_no", orderNo, "channel", channel, "trade_no", paid.TradeNo, "error", err)
		return err
	case err != nil:
		return err
	case out == order.OutcomePaid:
		s.log.Info("订单已支付", "order_no", orderNo, "channel", channel, "trade_no", paid.TradeNo)
	case out == order.OutcomeDuplicate:
		s.log.Error("订单重复支付，需要人工退款", "order_no", orderNo, "channel", channel, "trade_no", paid.TradeNo, "amount", paid.Amount)
	}
	return nil
}

// sync 主动查询渠道，已到账则确认。只处理已发起第三方支付的待支付订单。
func (s *Service) sync(ctx context.Context, o order.Order) error {
	if o.Status != order.StatusPending || !thirdParty(o.PayMethod) {
		return nil
	}
	set, err := s.gateways(ctx)
	if err != nil {
		return err
	}
	g, ok := set.gws[o.PayMethod]
	if !ok {
		return fmt.Errorf("订单 %s 的支付渠道 %s 没有配置", o.No, o.PayMethod)
	}
	paid, err := g.Query(ctx, o.No)
	if err != nil || paid == nil {
		return err
	}
	return s.confirm(ctx, o.PayMethod, o.No, paid)
}

// Refresh 前台轮询订单状态：待支付的订单顺便查询一次渠道，回调延迟或丢失时也能尽快确认。
func (s *Service) Refresh(ctx context.Context, userID int64, orderNo string) (order.Order, error) {
	o, err := s.orders.Get(ctx, userID, orderNo)
	if err != nil {
		return o, err
	}
	if o.Status == order.StatusPending && thirdParty(o.PayMethod) {
		if n, _ := s.limiter.Incr("pay:sync:"+o.No, syncInterval); n == 1 {
			if err := s.sync(ctx, o); err != nil {
				s.log.Warn("查询支付结果失败", "order_no", o.No, "error", err)
			}
			return s.orders.Get(ctx, userID, orderNo)
		}
	}
	return o, nil
}

// AdminSync 后台手动查询渠道补单。
func (s *Service) AdminSync(ctx context.Context, orderNo string) (order.Order, error) {
	o, err := s.orders.Get(ctx, 0, orderNo)
	if err != nil {
		return o, err
	}
	if o.Status != order.StatusPending {
		return o, nil
	}
	if !thirdParty(o.PayMethod) {
		return o, ErrNotStarted
	}
	if err := s.sync(ctx, o); err != nil {
		s.log.Error("后台补单查询失败", "order_no", o.No, "error", err)
		return o, ErrChannel
	}
	return s.orders.Get(ctx, 0, orderNo)
}

// closeOne 关闭一个待支付订单：先查一次渠道，已到账就确认，否则关闭渠道交易和本地订单。
func (s *Service) closeOne(ctx context.Context, o order.Order) error {
	if thirdParty(o.PayMethod) {
		set, err := s.gateways(ctx)
		if err != nil {
			return err
		}
		if g, ok := set.gws[o.PayMethod]; ok {
			paid, err := g.Query(ctx, o.No)
			if err != nil {
				return err
			}
			if paid != nil {
				return s.confirm(ctx, o.PayMethod, o.No, paid)
			}
			if err := g.Close(ctx, o.No); err != nil {
				s.log.Warn("关闭渠道交易失败", "order_no", o.No, "channel", o.PayMethod, "error", err)
			}
		}
	}
	_, err := s.orders.Close(ctx, o.ID)
	return err
}

// AdminClose 后台手动关闭待支付订单。
func (s *Service) AdminClose(ctx context.Context, adminID int64, ip, orderNo string) (order.Order, error) {
	o, err := s.orders.Get(ctx, 0, orderNo)
	if err != nil {
		return o, err
	}
	if o.Status != order.StatusPending {
		return o, order.ErrNotPending
	}
	if err := s.closeOne(ctx, o); err != nil {
		s.log.Error("后台关闭订单失败", "order_no", o.No, "error", err)
		return o, ErrChannel
	}
	if err := s.audit.Record(ctx, adminID, "order.close", "order", o.ID, map[string]any{"order_no": o.No}, ip); err != nil {
		return o, err
	}
	return s.orders.Get(ctx, 0, orderNo)
}

// Reconcile 补单任务：查询最近发起了第三方支付、还没确认到账的订单。
func (s *Service) Reconcile(ctx context.Context) error {
	list, err := s.orders.Awaiting(ctx, s.now().Add(-reconcileWindow), 200)
	if err != nil {
		return err
	}
	var errList []error
	for _, o := range list {
		if err := s.sync(ctx, o); err != nil {
			errList = append(errList, fmt.Errorf("订单 %s：%w", o.No, err))
		}
	}
	return errors.Join(errList...)
}

// CloseExpired 超时关闭任务。
func (s *Service) CloseExpired(ctx context.Context) error {
	list, err := s.orders.ExpiredPending(ctx, 200)
	if err != nil {
		return err
	}
	var errList []error
	for _, o := range list {
		if err := s.closeOne(ctx, o); err != nil {
			errList = append(errList, fmt.Errorf("订单 %s：%w", o.No, err))
		}
	}
	return errors.Join(errList...)
}

// MockChannelPaid 模拟用户已在渠道付款但还没有回调，只在模拟模式下生效。
func (s *Service) MockChannelPaid(channel, orderNo string, amount int64) {
	if g, ok := s.mocks[channel]; ok {
		g.pay(orderNo, amount)
	}
}

// MockPay 开发环境模拟用户在渠道完成付款，然后像补单一样查询确认。
func (s *Service) MockPay(ctx context.Context, userID int64, orderNo string) (order.Order, error) {
	if s.mocks == nil {
		return order.Order{}, ErrMockDisabled
	}
	o, err := s.orders.Get(ctx, userID, orderNo)
	if err != nil {
		return o, err
	}
	g, ok := s.mocks[o.PayMethod]
	if !ok {
		return o, ErrNotStarted
	}
	if o.Status != order.StatusPending {
		return o, order.ErrNotPending
	}
	g.pay(o.No, o.PayAmount)
	if err := s.sync(ctx, o); err != nil {
		return o, err
	}
	return s.orders.Get(ctx, userID, orderNo)
}
