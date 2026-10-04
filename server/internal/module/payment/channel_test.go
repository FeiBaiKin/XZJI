package payment

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
)

// 用本地生成的密钥模拟渠道签名，离线验证下单参数和回调验签，不访问真实渠道。

func genKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func pkcs8PEM(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func pubPEM(t *testing.T, k *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestAlipayGateway(t *testing.T) {
	appKey, aliKey := genKey(t), genKey(t)
	// 站长常把整个 PEM 文件粘进来，应能自动去掉头尾和换行
	g, err := newAlipay(AlipaySettings{
		AppID: "2021000000000001", Sandbox: true, KeyMode: AlipayKeyMode,
		PrivateKey: pkcs8PEM(t, appKey), AlipayPublicKey: pubPEM(t, aliKey),
	}, "https://api.test/api/v1/pay/notify/alipay")
	if err != nil {
		t.Fatal(err)
	}

	req := PayRequest{
		OrderNo: "20261004120000000001", Title: "VIP 年卡", Amount: 3900, Scene: ScenePC,
		ExpiresAt: time.Date(2026, 10, 4, 4, 30, 0, 0, time.UTC), ReturnURL: "https://www.test/pay/result?no=20261004120000000001",
	}
	act, err := g.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(act.URL)
	if err != nil || act.Kind != ActionRedirect || !strings.Contains(u.Host, "sandbox") {
		t.Fatalf("应返回沙箱收银台地址：%+v", act)
	}
	q := u.Query()
	var biz map[string]any
	_ = json.Unmarshal([]byte(q.Get("biz_content")), &biz)
	if q.Get("method") != "alipay.trade.page.pay" || q.Get("notify_url") != "https://api.test/api/v1/pay/notify/alipay" ||
		biz["out_trade_no"] != req.OrderNo || biz["total_amount"] != "39.00" || biz["time_expire"] != "2026-10-04 12:30:00" {
		t.Fatalf("下单参数不对：%v %v", q, biz)
	}
	// 用应用公钥验证请求签名：请求签名只排除 sign 本身
	bm := gopay.BodyMap{}
	for k := range q {
		if k != "sign" {
			bm.Set(k, q.Get(k))
		}
	}
	sum := sha256.Sum256([]byte(bm.EncodeAliPaySignParams()))
	sig, _ := base64.StdEncoding.DecodeString(q.Get("sign"))
	if err := rsa.VerifyPKCS1v15(&appKey.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatalf("请求签名验证失败：%v", err)
	}
	req.Scene = SceneMobile
	if act, _ := g.Create(context.Background(), req); !strings.Contains(act.URL, "alipay.trade.wap.pay") {
		t.Fatalf("手机端应使用手机网站支付：%s", act.URL)
	}

	notify := func(edit func(gopay.BodyMap), tamper func(url.Values)) (string, *Paid, error) {
		bm := gopay.BodyMap{}
		bm.Set("app_id", "2021000000000001").Set("out_trade_no", req.OrderNo).Set("trade_no", "2026100422001").
			Set("trade_status", "TRADE_SUCCESS").Set("total_amount", "39.00").Set("notify_time", "2026-10-04 12:01:00")
		if edit != nil {
			edit(bm)
		}
		// 异步通知的签名不含 sign 和 sign_type
		sign, err := alipay.GetRsaSign(bm, alipay.RSA2, aliKey)
		if err != nil {
			t.Fatal(err)
		}
		form := url.Values{}
		for k, v := range bm {
			form.Set(k, v.(string))
		}
		form.Set("sign", sign)
		form.Set("sign_type", "RSA2")
		if tamper != nil {
			tamper(form)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/pay/notify/alipay", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return g.Notify(r)
	}
	no, paid, err := notify(nil, nil)
	if err != nil || no != req.OrderNo || paid == nil || paid.Amount != 3900 || paid.TradeNo != "2026100422001" {
		t.Fatalf("合法回调解析不对：%s %+v %v", no, paid, err)
	}
	if _, _, err := notify(nil, func(f url.Values) { f.Set("total_amount", "0.01") }); !errors.Is(err, ErrBadNotify) {
		t.Fatalf("篡改金额应验签失败，得到 %v", err)
	}
	if _, _, err := notify(func(bm gopay.BodyMap) { bm.Set("app_id", "2021000000000999") }, nil); !errors.Is(err, ErrBadNotify) {
		t.Fatalf("别的应用的回调应拒绝，得到 %v", err)
	}
	if _, paid, err := notify(func(bm gopay.BodyMap) { bm.Set("trade_status", "WAIT_BUYER_PAY") }, nil); err != nil || paid != nil {
		t.Fatalf("未支付的通知应忽略：%+v %v", paid, err)
	}
}

func TestWechatNotify(t *testing.T) {
	mchKey, platKey := genKey(t), genKey(t)
	const apiV3Key = "0123456789abcdef0123456789abcdef"
	g, err := newWechat(context.Background(), WechatSettings{
		AppID: "wx0000000000000001", MchID: "1900000001", SerialNo: "MCHSERIAL", APIv3Key: apiV3Key,
		PrivateKey: pkcs8PEM(t, mchKey), PublicKeyID: "PUB_KEY_ID_TEST", PublicKey: pubPEM(t, platKey),
	}, "https://api.test/api/v1/pay/notify/wechat")
	if err != nil {
		t.Fatal(err)
	}
	if g.Supports(SceneMobile) {
		t.Fatal("没开通 H5 时手机端不可用")
	}

	notify := func(result map[string]any, serial string, tamper bool) (string, *Paid, error) {
		plain, _ := json.Marshal(result)
		block, _ := aes.NewCipher([]byte(apiV3Key))
		gcm, _ := cipher.NewGCM(block)
		nonce := "abcdefghijkl"
		ct := gcm.Seal(nil, []byte(nonce), plain, []byte("transaction"))
		body, _ := json.Marshal(map[string]any{
			"id": "EV-1", "create_time": "2026-10-04T12:01:00+08:00", "resource_type": "encrypt-resource", "event_type": "TRANSACTION.SUCCESS",
			"summary": "支付成功", "resource": map[string]any{
				"original_type": "transaction", "algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ct),
				"associated_data": "transaction", "nonce": nonce,
			},
		})
		ts, rnd := "1791100000", "NONCE123"
		sum := sha256.Sum256([]byte(ts + "\n" + rnd + "\n" + string(body) + "\n"))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, platKey, crypto.SHA256, sum[:])
		if tamper {
			body = []byte(strings.Replace(string(body), "支付成功", "支付失败", 1))
		}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/pay/notify/wechat", strings.NewReader(string(body)))
		r.Header.Set("Wechatpay-Timestamp", ts)
		r.Header.Set("Wechatpay-Nonce", rnd)
		r.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(sig))
		r.Header.Set("Wechatpay-Serial", serial)
		return g.Notify(r)
	}
	ok := map[string]any{
		"appid": "wx0000000000000001", "mchid": "1900000001", "out_trade_no": "20261004120000000002",
		"transaction_id": "4200000001", "trade_state": "SUCCESS", "amount": map[string]any{"total": 9900, "payer_total": 9900, "currency": "CNY"},
	}
	no, paid, err := notify(ok, "PUB_KEY_ID_TEST", false)
	if err != nil || no != "20261004120000000002" || paid == nil || paid.Amount != 9900 || paid.TradeNo != "4200000001" {
		t.Fatalf("合法回调解析不对：%s %+v %v", no, paid, err)
	}
	if _, _, err := notify(ok, "PUB_KEY_ID_TEST", true); !errors.Is(err, ErrBadNotify) {
		t.Fatalf("报文被篡改应验签失败，得到 %v", err)
	}
	if _, _, err := notify(ok, "OTHER_SERIAL", false); !errors.Is(err, ErrBadNotify) {
		t.Fatalf("未知的公钥 ID 应验签失败，得到 %v", err)
	}
	other := map[string]any{}
	for k, v := range ok {
		other[k] = v
	}
	other["mchid"] = "1900000999"
	if _, _, err := notify(other, "PUB_KEY_ID_TEST", false); !errors.Is(err, ErrBadNotify) {
		t.Fatalf("别的商户的回调应拒绝，得到 %v", err)
	}
	other["mchid"], other["trade_state"] = "1900000001", "NOTPAY"
	if _, paid, err := notify(other, "PUB_KEY_ID_TEST", false); err != nil || paid != nil {
		t.Fatalf("未支付的通知应忽略：%+v %v", paid, err)
	}
}
