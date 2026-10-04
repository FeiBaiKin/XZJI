package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-pay/gopay"
	wechat "github.com/go-pay/gopay/wechat/v3"
)

// wechatGateway 微信支付 v3：电脑端 Native 扫码，手机浏览器 H5 支付。
type wechatGateway struct {
	cfg       WechatSettings
	client    *wechat.ClientV3
	notifyURL string
}

func newWechatClient(_ context.Context, cfg WechatSettings) (*wechat.ClientV3, error) {
	if cfg.AppID == "" || cfg.MchID == "" || cfg.SerialNo == "" || cfg.APIv3Key == "" || cfg.PrivateKey == "" {
		return nil, errors.New("请填写 AppID、商户号、证书序列号、APIv3 密钥和商户私钥")
	}
	client, err := wechat.NewClientV3(cfg.MchID, cfg.SerialNo, cfg.APIv3Key, cfg.PrivateKey)
	if err != nil {
		// 不带原始错误：里面会包含私钥内容
		return nil, errors.New("商户私钥格式不正确，请粘贴 apiclient_key.pem 的完整内容")
	}
	if cfg.PublicKeyID != "" || cfg.PublicKey != "" {
		if cfg.PublicKeyID == "" || cfg.PublicKey == "" {
			return nil, errors.New("微信支付公钥 ID 和公钥需要同时填写")
		}
		if err := client.AutoVerifySignByPublicKey([]byte(cfg.PublicKey), cfg.PublicKeyID); err != nil {
			return nil, fmt.Errorf("微信支付公钥格式不正确：%w", err)
		}
	} else if err := client.AutoVerifySign(false); err != nil {
		// 没有填写微信支付公钥时下载平台证书验签；不自动刷新，网关缓存到期后重建客户端时重新下载
		return nil, fmt.Errorf("下载微信支付平台证书失败，请检查商户号、序列号和 APIv3 密钥：%w", err)
	}
	return client, nil
}

func newWechat(ctx context.Context, cfg WechatSettings, notifyURL string) (*wechatGateway, error) {
	client, err := newWechatClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &wechatGateway{cfg: cfg, client: client, notifyURL: notifyURL}, nil
}

func (g *wechatGateway) Supports(scene string) bool { return scene != SceneMobile || g.cfg.H5 }

func wxError(op string, code int, body string) error {
	return fmt.Errorf("微信支付%s：HTTP %d %s", op, code, body)
}

func (g *wechatGateway) Create(ctx context.Context, req PayRequest) (Action, error) {
	bm := make(gopay.BodyMap)
	bm.Set("appid", g.cfg.AppID).
		Set("description", truncateBytes(req.Title, 127)).
		Set("out_trade_no", req.OrderNo).
		Set("time_expire", req.ExpiresAt.In(beijing).Format("2006-01-02T15:04:05-07:00")).
		Set("notify_url", g.notifyURL).
		SetBodyMap("amount", func(b gopay.BodyMap) {
			b.Set("total", req.Amount).Set("currency", "CNY")
		})
	if req.Scene == SceneMobile {
		bm.SetBodyMap("scene_info", func(b gopay.BodyMap) {
			b.Set("payer_client_ip", req.ClientIP).SetBodyMap("h5_info", func(h gopay.BodyMap) { h.Set("type", "Wap") })
		})
		rsp, err := g.client.V3TransactionH5(ctx, bm)
		if err != nil {
			return Action{}, fmt.Errorf("微信支付 H5 下单：%w", err)
		}
		if rsp.Code != wechat.Success {
			return Action{}, wxError("H5 下单", rsp.Code, rsp.Error)
		}
		return Action{Kind: ActionRedirect, URL: rsp.Response.H5Url + "&redirect_url=" + url.QueryEscape(req.ReturnURL)}, nil
	}
	rsp, err := g.client.V3TransactionNative(ctx, bm)
	if err != nil {
		return Action{}, fmt.Errorf("微信支付 Native 下单：%w", err)
	}
	if rsp.Code != wechat.Success {
		return Action{}, wxError("Native 下单", rsp.Code, rsp.Error)
	}
	return Action{Kind: ActionQRCode, QRCode: rsp.Response.CodeUrl}, nil
}

func (g *wechatGateway) Query(ctx context.Context, orderNo string) (*Paid, error) {
	rsp, err := g.client.V3TransactionQueryOrder(ctx, wechat.OutTradeNo, orderNo)
	if err != nil {
		return nil, fmt.Errorf("微信支付查询订单 %s：%w", orderNo, err)
	}
	if rsp.Code == http.StatusNotFound {
		return nil, nil
	}
	if rsp.Code != wechat.Success {
		return nil, wxError("查询订单", rsp.Code, rsp.Error)
	}
	o := rsp.Response
	if o.TradeState != wechat.TradeStateSuccess || o.Amount == nil {
		return nil, nil
	}
	raw, _ := json.Marshal(o)
	return &Paid{TradeNo: o.TransactionId, Amount: int64(o.Amount.Total), Raw: string(raw)}, nil
}

func (g *wechatGateway) Close(ctx context.Context, orderNo string) error {
	rsp, err := g.client.V3TransactionCloseOrder(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("微信支付关闭订单 %s：%w", orderNo, err)
	}
	if rsp.Code != wechat.Success && rsp.Code != http.StatusNotFound {
		return wxError("关闭订单", rsp.Code, rsp.Error)
	}
	return nil
}

func (g *wechatGateway) Notify(r *http.Request) (string, *Paid, error) {
	n, err := wechat.V3ParseNotify(r)
	if err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	if err := n.VerifySignByPKMap(g.client.WxPublicKeyMap()); err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	res, err := n.DecryptPayCipherText(g.cfg.APIv3Key)
	if err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	if res.Mchid != g.cfg.MchID || res.Appid != g.cfg.AppID {
		return "", nil, fmt.Errorf("%w：商户号或 AppID 不匹配", ErrBadNotify)
	}
	if res.TradeState != wechat.TradeStateSuccess || res.Amount == nil {
		return res.OutTradeNo, nil, nil
	}
	raw, _ := json.Marshal(res)
	return res.OutTradeNo, &Paid{TradeNo: res.TransactionId, Amount: int64(res.Amount.Total), Raw: string(raw)}, nil
}
