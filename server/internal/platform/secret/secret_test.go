package secret

import (
	"path/filepath"
	"testing"
)

func TestSealOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k", "secret.key")
	b, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	sealed := b.Seal("-----BEGIN PRIVATE KEY-----")
	if sealed == "-----BEGIN PRIVATE KEY-----" || b.Seal("x") == b.Seal("x") {
		t.Fatalf("没有加密或没有随机数：%q", sealed)
	}
	// 重新加载同一个密钥文件后仍能解密
	b2, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b2.Open(sealed); err != nil || got != "-----BEGIN PRIVATE KEY-----" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	if got, err := b.Open(""); err != nil || got != "" || b.Seal("") != "" {
		t.Fatal("空字符串应原样返回")
	}
	other, _ := New(make([]byte, 32))
	if _, err := other.Open(sealed); err == nil {
		t.Fatal("换了密钥不应能解密")
	}
	if _, err := b.Open("plain"); err == nil {
		t.Fatal("未加密的内容应报错")
	}
}
