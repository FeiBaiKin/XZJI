package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// mockGateway 开发环境的模拟渠道：不连真实支付，前台点“模拟支付成功”后，查询即返回已到账。
// 回调接收未签名的 JSON {"orderNo","tradeNo","amount"}，只能在开发环境启用。
type mockGateway struct {
	channel string
	mu      sync.Mutex
	paid    map[string]int64
}

func newMock(channel string) *mockGateway {
	return &mockGateway{channel: channel, paid: map[string]int64{}}
}

func (g *mockGateway) Supports(string) bool { return true }

func (g *mockGateway) Create(_ context.Context, req PayRequest) (Action, error) {
	return Action{Kind: ActionQRCode, QRCode: fmt.Sprintf("XZJI-MOCK-%s:%s", g.channel, req.OrderNo), Mock: true}, nil
}

func (g *mockGateway) tradeNo(orderNo string) string { return "MOCK-" + g.channel + "-" + orderNo }

// pay 模拟用户在渠道完成付款。
func (g *mockGateway) pay(orderNo string, amount int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.paid[orderNo] = amount
}

func (g *mockGateway) Query(_ context.Context, orderNo string) (*Paid, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	amount, ok := g.paid[orderNo]
	if !ok {
		return nil, nil
	}
	return &Paid{TradeNo: g.tradeNo(orderNo), Amount: amount, Raw: "mock query"}, nil
}

func (g *mockGateway) Close(context.Context, string) error { return nil }

func (g *mockGateway) Notify(r *http.Request) (string, *Paid, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	var n struct {
		OrderNo string `json:"orderNo"`
		TradeNo string `json:"tradeNo"`
		Amount  int64  `json:"amount"`
	}
	if err := json.Unmarshal(body, &n); err != nil || n.OrderNo == "" {
		return "", nil, fmt.Errorf("%w：%v", ErrBadNotify, err)
	}
	if n.TradeNo == "" {
		n.TradeNo = g.tradeNo(n.OrderNo)
	}
	return n.OrderNo, &Paid{TradeNo: n.TradeNo, Amount: n.Amount, Raw: string(body)}, nil
}
