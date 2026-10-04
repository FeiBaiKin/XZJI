package wallet

import (
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		amount  int64
		want    int64
		err     error
	}{
		{"充值入账", 0, 4000, 4000, nil},
		{"购买扣款", 2850, -390, 2460, nil},
		{"刚好扣完", 390, -390, 0, nil},
		{"余额不足", 389, -390, 389, ErrInsufficient},
		{"零余额扣款", 0, -1, 0, ErrInsufficient},
		{"变动为 0", 100, 0, 100, ErrZeroAmount},
		{"入账到上限", MaxBalance - 100, 100, MaxBalance, nil},
		{"超过上限", MaxBalance - 100, 101, MaxBalance - 100, ErrBalanceLimit},
		{"超大金额不溢出", 1, 1<<62 + 1<<61, 1, ErrBalanceLimit},
		{"退款入账", 100, 990, 1090, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Apply(c.balance, c.amount)
			if got != c.want || !errors.Is(err, c.err) {
				t.Fatalf("Apply(%d, %d) = %d, %v；期望 %d, %v", c.balance, c.amount, got, err, c.want, c.err)
			}
		})
	}
}
