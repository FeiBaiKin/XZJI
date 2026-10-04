package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 支付场景：电脑扫码或跳转收银台，手机浏览器跳转渠道 H5 页面
const (
	ScenePC     = "pc"
	SceneMobile = "mobile"
)

// 发起支付后前台要做的事
const (
	ActionQRCode   = "qrcode"
	ActionRedirect = "redirect"
	ActionPaid     = "paid"
)

type PayRequest struct {
	OrderNo   string
	Title     string
	Amount    int64
	ExpiresAt time.Time
	Scene     string
	ClientIP  string
	NotifyURL string
	ReturnURL string
}

// Action 发起支付的结果：展示二维码、跳转到渠道页面，或者已经支付完成（余额支付）。
type Action struct {
	Kind   string
	QRCode string
	URL    string
	// Mock 模拟渠道，前台显示“模拟支付成功”按钮
	Mock bool
}

// Paid 渠道确认的到账信息。
type Paid struct {
	TradeNo string
	Amount  int64
	Raw     string
}

// Gateway 一个第三方支付渠道。
type Gateway interface {
	// Supports 是否支持该场景
	Supports(scene string) bool
	Create(ctx context.Context, req PayRequest) (Action, error)
	// Query 主动查询，未支付或渠道没有这笔交易时返回 nil
	Query(ctx context.Context, orderNo string) (*Paid, error)
	// Close 关闭渠道交易，渠道没有这笔交易时视为成功
	Close(ctx context.Context, orderNo string) error
	// Notify 解析并验签异步回调；不是支付成功的通知时 paid 为 nil
	Notify(r *http.Request) (orderNo string, paid *Paid, err error)
}

// ErrBadNotify 回调验签或解析失败。
var ErrBadNotify = errors.New("支付回调验签失败")

// yuan 分转换为渠道要求的元，保留两位小数。
func yuan(fen int64) string {
	return fmt.Sprintf("%d.%02d", fen/100, fen%100)
}

// parseYuan 把 "39.9" "39.90" "39" 解析为分，不经过浮点数。
func parseYuan(s string) (int64, error) {
	s = strings.TrimSpace(s)
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 {
		return 0, fmt.Errorf("金额格式不正确：%q", s)
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w < 0 {
		return 0, fmt.Errorf("金额格式不正确：%q", s)
	}
	frac = (frac + "00")[:2]
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式不正确：%q", s)
	}
	return w*100 + f, nil
}

// beijing 支付宝和微信接口里的本地时间都是北京时间。
var beijing = time.FixedZone("CST", 8*3600)
