package payment

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"xzji/internal/module/setting"
	"xzji/internal/platform/errs"
)

const settingKey = "payment"

// 支付宝密钥模式
const (
	AlipayKeyMode  = "key"
	AlipayCertMode = "cert"
)

// Settings 支付设置，存在站点设置的 payment 键里。私钥和 APIv3 密钥加密后保存。
type Settings struct {
	// NotifyBaseURL 接口的对外地址，异步回调发到 {NotifyBaseURL}/api/v1/pay/notify/{渠道}
	NotifyBaseURL string `json:"notifyBaseUrl"`
	// SiteURL 前台地址，支付完成后跳回 {SiteURL}/pay/result
	SiteURL string `json:"siteUrl"`
	// Balance 允许余额支付
	Balance bool           `json:"balance"`
	Alipay  AlipaySettings `json:"alipay"`
	Wechat  WechatSettings `json:"wechat"`
}

type AlipaySettings struct {
	Enabled bool `json:"enabled"`
	// Sandbox 使用支付宝沙箱环境
	Sandbox    bool   `json:"sandbox"`
	AppID      string `json:"appId"`
	PrivateKey string `json:"privateKey"`
	// KeyMode key：支付宝公钥；cert：公钥证书
	KeyMode         string `json:"keyMode"`
	AlipayPublicKey string `json:"alipayPublicKey"`
	AppCert         string `json:"appCert"`
	RootCert        string `json:"rootCert"`
	AlipayCert      string `json:"alipayCert"`
}

type WechatSettings struct {
	Enabled bool `json:"enabled"`
	// H5 已开通 H5 支付，手机浏览器可用
	H5       bool   `json:"h5"`
	AppID    string `json:"appId"`
	MchID    string `json:"mchId"`
	SerialNo string `json:"serialNo"`
	APIv3Key string `json:"apiV3Key"`
	// PrivateKey 商户 API 私钥 apiclient_key.pem
	PrivateKey string `json:"privateKey"`
	// PublicKeyID 和 PublicKey 是微信支付公钥；留空时自动下载平台证书验签
	PublicKeyID string `json:"publicKeyId"`
	PublicKey   string `json:"publicKey"`
}

// Secrets 需要加密保存、后台不回显的字段。
func (s *Settings) secrets() []*string {
	return []*string{&s.Alipay.PrivateKey, &s.Wechat.APIv3Key, &s.Wechat.PrivateKey}
}

func (s *Service) loadSettings(ctx context.Context) (Settings, error) {
	st := Settings{Balance: true, Alipay: AlipaySettings{KeyMode: AlipayKeyMode}}
	if err := s.settings.Get(ctx, settingKey, &st); err != nil && !errors.Is(err, setting.ErrNotFound) {
		return st, err
	}
	for _, p := range st.secrets() {
		plain, err := s.box.Open(*p)
		if err != nil {
			return st, fmt.Errorf("解密支付设置：%w", err)
		}
		*p = plain
	}
	return st, nil
}

// SecretState 后台显示哪些密钥已经填写。
type SecretState struct {
	AlipayPrivateKey bool
	WechatAPIv3Key   bool
	WechatPrivateKey bool
}

// AdminSettings 后台查看支付设置：密钥只告诉是否已填写，不返回内容。
func (s *Service) AdminSettings(ctx context.Context) (Settings, SecretState, error) {
	st, err := s.loadSettings(ctx)
	if err != nil {
		return st, SecretState{}, err
	}
	state := SecretState{AlipayPrivateKey: st.Alipay.PrivateKey != "", WechatAPIv3Key: st.Wechat.APIv3Key != "", WechatPrivateKey: st.Wechat.PrivateKey != ""}
	for _, p := range st.secrets() {
		*p = ""
	}
	return st, state, nil
}

func invalid(msg string) error { return errs.Invalid("INVALID_PAYMENT_SETTINGS", msg) }

func checkBaseURL(name, raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" {
		return "", invalid(name + "需要是 http(s):// 开头的域名，不带路径，例如 https://api.example.com")
	}
	return raw, nil
}

// SaveSettings 保存支付设置。密钥字段留空表示不修改。启用的渠道会用填写的参数实际创建一次客户端，参数有误时拒绝保存。
func (s *Service) SaveSettings(ctx context.Context, adminID int64, ip string, in Settings) error {
	old, err := s.loadSettings(ctx)
	if err != nil {
		return err
	}
	news, olds := in.secrets(), old.secrets()
	for i, p := range news {
		*p = strings.TrimSpace(*p)
		if *p == "" {
			*p = *olds[i]
		}
	}
	trim := func(ps ...*string) {
		for _, p := range ps {
			*p = strings.TrimSpace(*p)
		}
	}
	a, w := &in.Alipay, &in.Wechat
	trim(&a.AppID, &a.AlipayPublicKey, &a.AppCert, &a.RootCert, &a.AlipayCert, &w.AppID, &w.MchID, &w.SerialNo, &w.PublicKeyID, &w.PublicKey)
	if a.KeyMode != AlipayCertMode {
		a.KeyMode = AlipayKeyMode
	}
	if a.Enabled || w.Enabled {
		if in.NotifyBaseURL, err = checkBaseURL("接口地址", in.NotifyBaseURL); err != nil {
			return err
		}
		if in.SiteURL, err = checkBaseURL("前台地址", in.SiteURL); err != nil {
			return err
		}
	}
	if a.Enabled {
		if _, err := newAlipay(*a, s.notifyURL(in, "alipay")); err != nil {
			return invalid("支付宝参数有误：" + err.Error())
		}
	}
	if w.Enabled {
		if _, err := newWechatClient(ctx, *w); err != nil {
			return invalid("微信支付参数有误：" + err.Error())
		}
	}
	saved := in
	for _, p := range saved.secrets() {
		*p = s.box.Seal(*p)
	}
	if err := s.settings.Set(ctx, settingKey, saved); err != nil {
		return err
	}
	s.reset()
	detail := map[string]any{"alipay": a.Enabled, "wechat": w.Enabled, "balance": in.Balance}
	return s.audit.Record(ctx, adminID, "payment.settings", "setting", 0, detail, ip)
}
