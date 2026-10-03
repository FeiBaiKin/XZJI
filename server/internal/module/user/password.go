package user

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id 参数取 OWASP 推荐的最低配置（19 MiB、2 轮、1 线程），兼顾 1 核 1G 的小服务器。
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var errBadHash = errors.New("密码哈希格式不正确")

// Hasher 计算和校验 argon2id 密码哈希，并限制同时计算的数量，避免登录高峰时内存暴涨。
type Hasher struct {
	sem chan struct{}
}

func NewHasher(concurrency int) *Hasher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Hasher{sem: make(chan struct{}, concurrency)}
}

func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h.sem <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-h.sem
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify 按哈希中记录的参数重新计算，以后调整参数也能校验旧密码。
func (h *Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var mem, iter uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iter, &threads); err != nil {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, errBadHash
	}
	h.sem <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, iter, mem, threads, uint32(len(want)))
	<-h.sem
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
