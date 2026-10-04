package payment

import "testing"

func TestYuan(t *testing.T) {
	cases := map[int64]string{0: "0.00", 1: "0.01", 99: "0.99", 100: "1.00", 3900: "39.00", 1990: "19.90", 123456: "1234.56"}
	for fen, want := range cases {
		if got := yuan(fen); got != want {
			t.Errorf("yuan(%d) = %q，期望 %q", fen, got, want)
		}
	}
}

func TestParseYuan(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"39.00", 3900, true},
		{"39.9", 3990, true},
		{"39", 3900, true},
		{"0.01", 1, true},
		{" 1234.56 ", 123456, true},
		{"1.234", 0, false},
		{"", 0, false},
		{".5", 0, false},
		{"-1.00", 0, false},
		{"abc", 0, false},
		{"1.x", 0, false},
	}
	for _, c := range cases {
		got, err := parseYuan(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("parseYuan(%q) = %d, %v；期望 %d，成功=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestTruncateBytes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abc", 5, "abc"},
		{"abc", 2, "ab"},
		{"中文标题", 6, "中文"},
		{"中文标题", 7, "中文"},
		{"中文标题", 2, ""},
	}
	for _, c := range cases {
		if got := truncateBytes(c.in, c.n); got != c.want {
			t.Errorf("truncateBytes(%q, %d) = %q，期望 %q", c.in, c.n, got, c.want)
		}
	}
}
