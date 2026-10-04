package access

import "testing"

func TestParseCond(t *testing.T) {
	cases := []struct {
		in   string
		want Cond
		ok   bool
	}{
		{"public", Cond{Kind: KindPublic}, true},
		{"login", Cond{Kind: KindLogin}, true},
		{"comment", Cond{Kind: KindComment}, true},
		{"paid", Cond{Kind: KindPaid}, true},
		{"member:2", Cond{Kind: KindMember, Rank: 2}, true},
		{"member:99", Cond{Kind: KindMember, Rank: 99}, true},
		{"member:0", Cond{}, false},
		{"member:100", Cond{}, false},
		{"member:", Cond{}, false},
		{"member", Cond{}, false},
		{"vip", Cond{}, false},
		{"", Cond{}, false},
	}
	for _, c := range cases {
		got, err := ParseCond(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("ParseCond(%q) = %+v, %v；期望 %+v，成功=%v", c.in, got, err, c.want, c.ok)
		}
		if c.ok && got.String() != c.in {
			t.Errorf("Cond(%q).String() = %q", c.in, got.String())
		}
	}
}

func TestCheck(t *testing.T) {
	anon := Viewer{}
	user := Viewer{UserID: 1}
	commented := Viewer{UserID: 1, Commented: true}
	bought := Viewer{UserID: 1, Purchased: true}
	vip1 := Viewer{UserID: 1, MemberRank: 1}
	vip2 := Viewer{UserID: 1, MemberRank: 2}
	admin := Viewer{UserID: 9, Privileged: true}

	free2 := Rule{Price: 990, MemberFreeRank: 2}
	paidOnly := Rule{Price: 990}
	memberOnly := Rule{MemberFreeRank: 1}

	lock := func(k Kind, rank int) Decision { return Decision{Lock: &Lock{Reason: k, Rank: rank}} }
	allow := Decision{Allowed: true}
	memberFree := Decision{Allowed: true, MemberFree: true}

	cases := []struct {
		name string
		cond string
		rule Rule
		v    Viewer
		want Decision
	}{
		{"公开内容匿名可见", "public", free2, anon, allow},
		{"登录可见：未登录", "login", Rule{}, anon, lock(KindLogin, 0)},
		{"登录可见：已登录", "login", Rule{}, user, allow},
		{"评论可见：未登录", "comment", Rule{}, anon, lock(KindComment, 0)},
		{"评论可见：评论未通过", "comment", Rule{}, user, lock(KindComment, 0)},
		{"评论可见：评论已通过", "comment", Rule{}, commented, allow},
		{"付费：未购买的普通用户", "paid", free2, user, lock(KindPaid, 2)},
		{"付费：已购买", "paid", free2, bought, allow},
		{"付费：会员等级不够", "paid", free2, vip1, lock(KindPaid, 2)},
		{"付费：会员等级够，算会员免费", "paid", free2, vip2, memberFree},
		{"付费：会员也不免费", "paid", paidOnly, vip2, lock(KindPaid, 0)},
		{"付费：已购买的会员不算会员免费", "paid", free2, Viewer{UserID: 1, MemberRank: 2, Purchased: true}, allow},
		{"付费：只能会员免费，匿名", "paid", memberOnly, anon, lock(KindPaid, 1)},
		{"会员可见：非会员", "member:2", Rule{}, user, lock(KindMember, 2)},
		{"会员可见：等级不够", "member:2", Rule{}, vip1, lock(KindMember, 2)},
		{"会员可见：等级够", "member:2", Rule{}, vip2, memberFree},
		{"会员可见：已购买也不行", "member:2", free2, bought, lock(KindMember, 2)},
		{"管理员或作者始终可见", "member:2", Rule{}, admin, allow},
		{"管理员看付费内容", "paid", paidOnly, admin, allow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cond, err := ParseCond(c.cond)
			if err != nil {
				t.Fatal(err)
			}
			got := Check(cond, c.rule, c.v)
			if got.Allowed != c.want.Allowed || got.MemberFree != c.want.MemberFree || (got.Lock == nil) != (c.want.Lock == nil) ||
				(got.Lock != nil && *got.Lock != *c.want.Lock) {
				t.Fatalf("Check = %+v（锁 %+v），期望 %+v（锁 %+v）", got, got.Lock, c.want, c.want.Lock)
			}
		})
	}
}

func TestPrice(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		v    Viewer
		want int64
		ok   bool
	}{
		{"没有设置价格", Rule{MemberFreeRank: 1}, Viewer{UserID: 1}, 0, false},
		{"普通用户原价", Rule{Price: 990, MemberDiscount: true}, Viewer{UserID: 1, DiscountRate: 100}, 990, true},
		{"未登录按原价显示", Rule{Price: 990, MemberDiscount: true}, Viewer{DiscountRate: 100}, 990, true},
		{"会员 8 折", Rule{Price: 990, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 80}, 792, true},
		{"会员不享受折扣", Rule{Price: 990}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 80}, 990, true},
		{"四舍五入：0.5 分进位", Rule{Price: 1, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 50}, 1, true},
		{"四舍五入：395 × 0.85 = 335.75", Rule{Price: 395, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 85}, 336, true},
		{"四舍五入：395 × 0.81 = 319.95", Rule{Price: 395, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 81}, 320, true},
		{"四舍五入：333 × 0.5 = 166.5", Rule{Price: 333, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 50}, 167, true},
		{"舍去：331 × 0.3 = 99.3", Rule{Price: 331, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 30}, 99, true},
		{"最低 1 分", Rule{Price: 1, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 1}, 1, true},
		{"会员等级够免费，不需要购买", Rule{Price: 990, MemberFreeRank: 2, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 2, DiscountRate: 70}, 0, false},
		{"等级不够免费，享受折扣", Rule{Price: 990, MemberFreeRank: 2, MemberDiscount: true}, Viewer{UserID: 1, MemberRank: 1, DiscountRate: 80}, 792, true},
		{"已经购买过", Rule{Price: 990}, Viewer{UserID: 1, Purchased: true}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Price(c.rule, c.v)
			if got != c.want || ok != c.ok {
				t.Fatalf("Price = %d, %v；期望 %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}
