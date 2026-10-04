package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
)

// alipayGateway 支付宝电脑网站支付和手机网站支付。
type alipayGateway struct {
	cfg       AlipaySettings
	client    *alipay.Client
	notifyURL string
}

// bareKey 去掉 PEM 头尾和空白：SDK 只接受开放平台给出的单行密钥，站长可能粘贴整个 PEM 文件。
func bareKey(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); !strings.HasPrefix(line, "-----") {
			b.WriteString(strings.Join(strings.Fields(line), ""))
		}
	}
	return b.String()
}

func newAlipay(cfg AlipaySettings, notifyURL string) (*alipayGateway, error) {
	cfg.PrivateKey, cfg.AlipayPublicKey = bareKey(cfg.PrivateKey), bareKey(cfg.AlipayPublicKey)
	if cfg.AppID == "" || cfg.PrivateKey == "" {
		return nil, errors.New("请填写 AppID 和应用私钥")
	}
	client, err := alipay.NewClient(cfg.AppID, cfg.PrivateKey, !cfg.Sandbox)
	if err != nil {
		// 不带原始错误：里面会包含私钥内容
		return nil, errors.New("应用私钥格式不正确，请粘贴 PKCS1 或 PKCS8 格式的 RSA2 私钥")
	}
	client.SetLocation("Asia/Shanghai")
	switch cfg.KeyMode {
	case AlipayCertMode:
		if cfg.AppCert == "" || cfg.RootCert == "" || cfg.AlipayCert == "" {
			return nil, errors.New("证书模式需要填写应用公钥证书、支付宝根证书和支付宝公钥证书")
		}
		if err := client.SetCertSnByContent([]byte(cfg.AppCert), []byte(cfg.RootCert), []byte(cfg.AlipayCert)); err != nil {
			return nil, fmt.Errorf("证书内容不正确：%w", err)
		}
		client.AutoVerifySign([]byte(cfg.AlipayCert))
	default:
		if cfg.AlipayPublicKey == "" {
			return nil, errors.New("请填写支付宝公钥")
		}
	}
	return &alipayGateway{cfg: cfg, client: client, notifyURL: notifyURL}, nil
}

func (g *alipayGateway) Supports(string) bool { return true }

func (g *alipayGateway) Create(ctx context.Context, req PayRequest) (Action, error) {
	bm := make(gopay.BodyMap)
	bm.Set("subject", truncateBytes(req.Title, 256)).
		Set("out_trade_no", req.OrderNo).
		Set("total_amount", yuan(req.Amount)).
		Set("time_expire", req.ExpiresAt.In(beijing).Format("2006-01-02 15:04:05")).
		Set("notify_url", g.notifyURL).
		Set("return_url", req.ReturnURL)
	var payURL string
	var err error
	if req.Scene == SceneMobile {
		bm.Set("quit_url", req.ReturnURL)
		payURL, err = g.client.TradeWapPay(ctx, bm)
	} else {
		payURL, err = g.client.TradePagePay(ctx, bm)
	}
	if err != nil {
		return Action{}, fmt.Errorf("支付宝下单：%w", err)
	}
	return Action{Kind: ActionRedirect, URL: payURL}, nil
}

func tradeNotExist(err error) bool {
	be, ok := alipay.IsBizError(err)
	return ok && be.SubCode == "ACQ.TRADE_NOT_EXIST"
}

func alipayPaid(status string) bool { return status == "TRADE_SUCCESS" || status == "TRADE_FINISHED" }

func (g *alipayGateway) Query(ctx context.Context, orderNo string) (*Paid, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderNo)
	rsp, err := g.client.TradeQuery(ctx, bm)
	if tradeNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("支付宝查询订单 %s：%w", orderNo, err)
	}
	// 证书模式由客户端自动验签，公钥模式在这里验
	if g.cfg.KeyMode != AlipayCertMode {
		if ok, err := alipay.VerifySyncSign(g.cfg.AlipayPublicKey, rsp.SignData, rsp.Sign); err != nil || !ok {
			return nil, fmt.Errorf("支付宝查询订单 %s：响应验签失败 %v", orderNo, err)
		}
	}
	if !alipayPaid(rsp.Response.TradeStatus) {
		return nil, nil
	}
	amount, err := parseYuan(rsp.Response.TotalAmount)
	if err != nil {
		return nil, err
	}
	return &Paid{TradeNo: rsp.Response.TradeNo, Amount: amount, Raw: rsp.SignData}, nil
}

func (g *alipayGateway) Close(ctx context.Context, orderNo string) error {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderNo)
	if _, err := g.client.TradeClose(ctx, bm); err != nil && !tradeNotExist(err) {
		return fmt.Errorf("支付宝关闭订单 %s：%w", orderNo, err)
	}
	return nil
}

func (g *alipayGateway) Notify(r *http.Request) (string, *Paid, error) {
	bm, err := alipay.ParseNotifyToBodyMap(r)
	if err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	raw := bm.EncodeURLParams()
	var ok bool
	if g.cfg.KeyMode == AlipayCertMode {
		ok, err = alipay.VerifySignWithCert([]byte(g.cfg.AlipayCert), bm)
	} else {
		ok, err = alipay.VerifySign(g.cfg.AlipayPublicKey, bm)
	}
	if err != nil || !ok {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	if bm.GetString("app_id") != g.cfg.AppID {
		return "", nil, fmt.Errorf("%w：app_id 不匹配", ErrBadNotify)
	}
	orderNo := bm.GetString("out_trade_no")
	if !alipayPaid(bm.GetString("trade_status")) {
		return orderNo, nil, nil
	}
	amount, err := parseYuan(bm.GetString("total_amount"))
	if err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	return orderNo, &Paid{TradeNo: bm.GetString("trade_no"), Amount: amount, Raw: raw}, nil
}

// truncateBytes 按字节截断且不截断半个字符，渠道对标题长度按字节限制。
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := 0
	for i := range s {
		if i > n {
			break
		}
		cut = i
	}
	return s[:cut]
}
