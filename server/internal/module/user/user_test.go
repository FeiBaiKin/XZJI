package user

import (
	"errors"
	"strings"
	"testing"
)

func TestHasher(t *testing.T) {
	h := NewHasher(2)
	hash, err := h.Hash("abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("哈希格式不符：%s", hash)
	}
	if ok, err := h.Verify("abc12345", hash); err != nil || !ok {
		t.Fatalf("正确密码校验失败：%v %v", ok, err)
	}
	if ok, _ := h.Verify("abc12346", hash); ok {
		t.Fatal("错误密码不应通过")
	}
	if _, err := h.Verify("abc12345", "$bcrypt$xxx"); err == nil {
		t.Fatal("非 argon2id 哈希应报错")
	}
	other, _ := h.Hash("abc12345")
	if other == hash {
		t.Fatal("同一密码两次哈希应因盐不同而不同")
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{" You@Example.COM ", "you@example.com", true},
		{"a.b+tag@sub.example.cn", "a.b+tag@sub.example.cn", true},
		{"no-at-sign", "", false},
		{"user@localhost", "", false},
		{"张三 <a@b.com>", "", false},
		{"a@b.com\r\nBcc: x@y.com", "", false},
		{strings.Repeat("a", 190) + "@b.com", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeEmail(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeEmail(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"abc12345", true},
		{"中文密码abc1", true},
		{"abcdefgh", false},
		{"12345678", false},
		{"abc1234", false},
		{strings.Repeat("a1", 65), false},
	}
	for _, tt := range tests {
		err := ValidatePassword(tt.in)
		if (err == nil) != tt.ok {
			t.Errorf("ValidatePassword(%q) = %v", tt.in, err)
		}
		if err != nil && !errors.Is(err, ErrWeakPassword) {
			t.Errorf("应返回 ErrWeakPassword，得到 %v", err)
		}
	}
}

func TestNickname(t *testing.T) {
	if _, err := NormalizeNickname(" 林间小路 "); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"a", strings.Repeat("长", 21), "含\x00控制符"} {
		if _, err := NormalizeNickname(bad); err == nil {
			t.Errorf("NormalizeNickname(%q) 应报错", bad)
		}
	}
	tests := map[string]string{
		"linjian@qq.com":                  "linjian",
		"a@qq.com":                        "用户a",
		strings.Repeat("x", 30) + "@a.cn": strings.Repeat("x", 20),
	}
	for email, want := range tests {
		if got := DefaultNickname(email); got != want {
			t.Errorf("DefaultNickname(%q) = %q, want %q", email, got, want)
		}
	}
}
