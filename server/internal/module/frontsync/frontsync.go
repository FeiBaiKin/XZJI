// Package frontsync 在内容变更后通知各前台服务器清空页面缓存。
// 通知带 HMAC 签名和时间戳，防止被伪造或重放。
package frontsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"xzji/internal/platform/cache"
)

const (
	// Path 前台清除缓存的接口
	Path = "/_internal/purge"
	// HeaderTimestamp / HeaderSignature 通知请求带的签名头
	HeaderTimestamp = "X-Purge-Timestamp"
	HeaderSignature = "X-Purge-Signature"

	// coalesceWindow 1 秒内的多次清除合并成一次通知
	coalesceWindow = time.Second
	requestTimeout = 3 * time.Second
)

// Signature 计算通知签名：hex(HMAC-SHA256(secret, timestamp))。
func Signature(secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d", timestamp)
	return hex.EncodeToString(mac.Sum(nil))
}

type Service struct {
	store  cache.Cache
	urls   []string
	secret string
	log    *slog.Logger
	client *http.Client
	now    func() time.Time

	mu    sync.Mutex
	timer *time.Timer
}

// NewService urls 是需要通知的前台服务器地址，末尾带不带 / 都可以。
func NewService(store cache.Cache, urls []string, secret string, log *slog.Logger) *Service {
	return &Service{
		store: store, urls: urls, secret: secret, log: log, now: time.Now,
		client: &http.Client{Timeout: requestTimeout},
	}
}

// Purge 先清后端缓存，再通知各前台服务器。顺序不能反：反了前台会拿后端的旧数据重新渲染并缓存。
func (s *Service) Purge() {
	s.store.Purge()
	if len(s.urls) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		return // 已有待发的通知，它会把这次变更一起带出去
	}
	s.timer = time.AfterFunc(coalesceWindow, s.notify)
}

// notify 向每台前台服务器发送清除请求；失败只记日志，前台有 10 分钟兜底过期。
func (s *Service) notify() {
	s.mu.Lock()
	s.timer = nil
	s.mu.Unlock()

	ts := s.now().Unix()
	sig := Signature(s.secret, ts)
	for _, u := range s.urls {
		go s.send(u, ts, sig)
	}
}

func (s *Service) send(base string, ts int64, sig string) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(base, "/")+Path, nil)
	if err != nil {
		s.log.Warn("前台清除缓存的通知地址不正确", "url", base, "error", err)
		return
	}
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(HeaderSignature, sig)

	res, err := s.client.Do(req)
	if err != nil {
		s.log.Warn("通知前台清除缓存失败", "url", base, "error", err)
		return
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusNoContent {
		s.log.Warn("前台清除缓存返回异常状态", "url", base, "status", res.StatusCode)
	}
}
