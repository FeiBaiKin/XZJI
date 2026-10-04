// Package secret 加密存进数据库的敏感配置（如支付私钥），只拿到数据库备份也解不出明文。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const prefix = "enc:v1:"

type Box struct {
	aead cipher.AEAD
}

// New 用 32 字节密钥创建 AES-256-GCM 加密器。
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("密钥必须是 32 字节")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// LoadOrCreate 读取密钥文件（base64 编码的 32 字节），不存在时生成。
// 密钥文件丢失后已加密的配置无法解密，需要在后台重新填写。
func LoadOrCreate(path string) (*Box, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("创建密钥目录：%w", err)
		}
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)+"\n"), 0o600); err != nil {
			return nil, fmt.Errorf("写入密钥文件 %s：%w", path, err)
		}
		return New(key)
	}
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件 %s：%w", path, err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, fmt.Errorf("密钥文件 %s 格式不正确：%w", path, err)
	}
	return New(key)
}

// Seal 加密，空字符串原样返回。
func (b *Box) Seal(plain string) string {
	if plain == "" {
		return ""
	}
	nonce := make([]byte, b.aead.NonceSize())
	_, _ = rand.Read(nonce)
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.StdEncoding.EncodeToString(out)
}

// Open 解密 Seal 的结果，空字符串原样返回。
func (b *Box) Open(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	raw, ok := strings.CutPrefix(s, prefix)
	if !ok {
		return "", errors.New("不是加密内容")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(data) < b.aead.NonceSize() {
		return "", errors.New("加密内容已损坏")
	}
	n := b.aead.NonceSize()
	plain, err := b.aead.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return "", errors.New("解密失败，密钥文件可能已更换")
	}
	return string(plain), nil
}
