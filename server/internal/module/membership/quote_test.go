package membership

import (
	"errors"
	"testing"
	"time"
)

func TestBasisOf(t *testing.T) {
	month := Plan{ID: 1, DurationDays: 30, Price: 990, Status: StatusActive}
	year := Plan{ID: 2, DurationDays: 365, Price: 3900, Status: StatusActive, Sort: 1}
	forever := Plan{ID: 3, DurationDays: 0, Price: 9900, Status: StatusActive, Sort: 2}
	quarter := Plan{ID: 4, DurationDays: 90, Price: 2500, Status: StatusActive}
	promoYear := Plan{ID: 5, DurationDays: 365, Price: 2900, Status: StatusActive, Sort: 5}
	disabledYear := Plan{ID: 6, DurationDays: 365, Price: 5000, Status: StatusDisabled}
	disabledForever := Plan{ID: 7, DurationDays: 0, Price: 19900, Status: StatusDisabled}

	cases := []struct {
		name  string
		plans []Plan
		want  Basis
	}{
		{"取年卡和永久价", []Plan{month, year, forever}, Basis{Price: 3900, Days: 365, PermanentPrice: 9900}},
		{"没有年卡取时长最长的有限期套餐", []Plan{month, quarter, forever}, Basis{Price: 2500, Days: 90, PermanentPrice: 9900}},
		{"多个年卡取排序靠前的", []Plan{promoYear, year}, Basis{Price: 3900, Days: 365}},
		{"启用的年卡优先于停用的", []Plan{disabledYear, month}, Basis{Price: 990, Days: 30}},
		{"只有停用的年卡时用停用的", []Plan{disabledYear, disabledForever}, Basis{Price: 5000, Days: 365, PermanentPrice: 19900}},
		{"只有永久套餐", []Plan{forever}, Basis{PermanentPrice: 9900}},
		{"没有套餐", nil, Basis{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BasisOf(c.plans); got != c.want {
				t.Fatalf("BasisOf = %+v，期望 %+v", got, c.want)
			}
		})
	}
}

func TestCompute(t *testing.T) {
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	at := func(days int, hours int) *time.Time {
		v := now.Add(time.Duration(days)*day + time.Duration(hours)*time.Hour)
		return &v
	}
	// VIP：月卡 9.9、年卡 39、永久 99；SVIP：月卡 19.9、年卡 99、永久 199
	vipBasis := Basis{Price: 3900, Days: 365, PermanentPrice: 9900}
	svipBasis := Basis{Price: 9900, Days: 365, PermanentPrice: 19900}
	vipYear := Plan{DurationDays: 365, Price: 3900}
	vipForever := Plan{DurationDays: 0, Price: 9900}
	svipMonth := Plan{DurationDays: 30, Price: 1990}
	svipYear := Plan{DurationDays: 365, Price: 9900}
	svipForever := Plan{DurationDays: 0, Price: 19900}
	none := Current{}
	vip160 := Current{Rank: 1, ExpiresAt: at(160, 5)}
	vipForeverCur := Current{Rank: 1}
	svip30 := Current{Rank: 2, ExpiresAt: at(30, 0)}

	cases := []struct {
		name     string
		cur      Current
		plan     Plan
		rank     int
		curBasis Basis
		newBasis Basis
		want     Quote
		err      error
	}{
		{
			name: "新开通年卡", cur: none, plan: vipYear, rank: 1, newBasis: vipBasis,
			want: Quote{Kind: KindNew, Price: 3900, PayAmount: 3900, ExpiresAt: at(365, 0)},
		},
		{
			name: "新开通永久", cur: none, plan: vipForever, rank: 1, newBasis: vipBasis,
			want: Quote{Kind: KindNew, Price: 9900, PayAmount: 9900},
		},
		{
			name: "同等级续费：在原到期时间上叠加", cur: vip160, plan: vipYear, rank: 1, curBasis: vipBasis, newBasis: vipBasis,
			want: Quote{Kind: KindRenew, Price: 3900, PayAmount: 3900, ExpiresAt: at(160+365, 5)},
		},
		{
			// 160 × 39 ÷ 365 = 17.095 → 17.10 元；17.10 ÷ (99 ÷ 365) = 63.04 → 63 天
			name: "升级到更高等级的年卡：剩余价值折算成天数", cur: vip160, plan: svipYear, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 9900, PayAmount: 9900, RemainingDays: 160, RemainingValue: 1710, ConvertedDays: 63, ExpiresAt: at(365+63, 0)},
		},
		{
			name: "升级到更高等级的月卡：按新等级年卡的日均价折算", cur: vip160, plan: svipMonth, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 1990, PayAmount: 1990, RemainingDays: 160, RemainingValue: 1710, ConvertedDays: 63, ExpiresAt: at(30+63, 0)},
		},
		{
			name: "升级到更高等级的永久套餐：剩余价值抵扣价格", cur: vip160, plan: svipForever, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 19900, Deduct: 1710, PayAmount: 18190, RemainingDays: 160, RemainingValue: 1710},
		},
		{
			name: "同等级有限期升永久：剩余价值抵扣", cur: vip160, plan: vipForever, rank: 1, curBasis: vipBasis, newBasis: vipBasis,
			want: Quote{Kind: KindUpgrade, Price: 9900, Deduct: 1710, PayAmount: 8190, RemainingDays: 160, RemainingValue: 1710},
		},
		{
			name: "剩余不足一天按 0 天算", cur: Current{Rank: 1, ExpiresAt: at(0, 23)}, plan: svipYear, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 9900, PayAmount: 9900, ExpiresAt: at(365, 0)},
		},
		{
			name: "原等级没有有限期套餐，剩余价值为 0", cur: vip160, plan: svipYear, rank: 2, curBasis: Basis{PermanentPrice: 9900}, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 9900, PayAmount: 9900, RemainingDays: 160, ExpiresAt: at(365, 0)},
		},
		{
			name: "新等级没有有限期套餐，不折算天数", cur: vip160, plan: svipMonth, rank: 2, curBasis: vipBasis, newBasis: Basis{},
			want: Quote{Kind: KindUpgrade, Price: 1990, PayAmount: 1990, RemainingDays: 160, RemainingValue: 1710, ExpiresAt: at(30, 0)},
		},
		{
			name: "剩余价值超过永久价，最少付 1 分", cur: Current{Rank: 1, ExpiresAt: at(3650, 0)}, plan: Plan{DurationDays: 0, Price: 5000}, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 5000, Deduct: 4999, PayAmount: 1, RemainingDays: 3650, RemainingValue: 39000},
		},
		{
			name: "永久会员升级到更高等级的永久：补差价", cur: vipForeverCur, plan: svipForever, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 19900, Deduct: 9900, PayAmount: 10000},
		},
		{
			name: "永久会员的原等级没有永久套餐：不抵扣", cur: vipForeverCur, plan: svipForever, rank: 2, curBasis: Basis{Price: 3900, Days: 365}, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 19900, PayAmount: 19900},
		},
		{
			name: "永久差价不足 1 分时付 1 分", cur: vipForeverCur, plan: Plan{DurationDays: 0, Price: 9000}, rank: 2, curBasis: vipBasis, newBasis: svipBasis,
			want: Quote{Kind: KindUpgrade, Price: 9000, Deduct: 8999, PayAmount: 1},
		},
		{name: "永久会员不能买有限期套餐", cur: vipForeverCur, plan: svipYear, rank: 2, curBasis: vipBasis, newBasis: svipBasis, err: ErrPermanentOnly},
		{name: "永久会员不能再买同等级永久", cur: vipForeverCur, plan: vipForever, rank: 1, curBasis: vipBasis, newBasis: vipBasis, err: ErrAlreadyPermanent},
		{name: "有效期内不能买低等级", cur: svip30, plan: vipYear, rank: 1, curBasis: svipBasis, newBasis: vipBasis, err: ErrDowngrade},
		{
			name: "已过期（rank 为 0）可以买低等级", cur: none, plan: vipYear, rank: 1, newBasis: vipBasis,
			want: Quote{Kind: KindNew, Price: 3900, PayAmount: 3900, ExpiresAt: at(365, 0)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Compute(c.cur, c.plan, c.rank, c.curBasis, c.newBasis, now)
			if c.err != nil {
				if !errors.Is(err, c.err) {
					t.Fatalf("期望错误 %v，得到 %v（%+v）", c.err, err, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sameTime := (got.ExpiresAt == nil) == (c.want.ExpiresAt == nil) && (got.ExpiresAt == nil || got.ExpiresAt.Equal(*c.want.ExpiresAt))
			g, w := got, c.want
			g.ExpiresAt, w.ExpiresAt = nil, nil
			if g != w || !sameTime {
				t.Fatalf("Compute =\n  %+v 到期 %v\n期望\n  %+v 到期 %v", g, got.ExpiresAt, w, c.want.ExpiresAt)
			}
		})
	}
}
