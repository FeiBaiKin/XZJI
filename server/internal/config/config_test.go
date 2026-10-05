package config

import (
	"strings"
	"testing"
)

// prodConfig 一份能通过校验的生产配置，用例只改其中一项。
func prodConfig() *Config {
	c := defaults()
	c.Env = EnvProd
	c.DB.DSN = "xzji:password@tcp(127.0.0.1:3306)/xzji"
	c.Mail.Driver = "smtp"
	c.Mail.SMTP.Host = "smtp.example.com"
	c.Mail.SMTP.From = "XZJI <noreply@example.com>"
	c.Media.BaseURL = "https://img.example.com"
	c.Media.Serve = false
	c.Auth.CookieSecure = true
	c.CORS.AllowOrigins = []string{"https://www.example.com"}
	c.Frontend.PurgeSecret = "secret"
	return c
}

func TestValidateProd(t *testing.T) {
	cases := []struct {
		name    string
		change  func(*Config)
		wantErr string
	}{
		{"生产配置齐全", func(*Config) {}, ""},
		{"不能开模拟支付", func(c *Config) { c.Pay.Mock = true }, "pay.mock"},
		{"必须开 cookie_secure", func(c *Config) { c.Auth.CookieSecure = false }, "cookie_secure"},
		{"必须有 SMTP", func(c *Config) { c.Mail.Driver = "log" }, "SMTP"},
		{"必须有清除密钥", func(c *Config) { c.Frontend.PurgeSecret = "" }, "purge_secret"},
		{"CORS 不能是 http", func(c *Config) { c.CORS.AllowOrigins = []string{"http://www.example.com"} }, "https"},
		{"CORS 混进 http 也报错", func(c *Config) {
			c.CORS.AllowOrigins = []string{"https://www.example.com", "http://old.example.com"}
		}, "https"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := prodConfig()
			c.change(cfg)
			err := cfg.validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("不该报错，实际：%v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("期望包含 %q 的错误，实际：%v", c.wantErr, err)
			}
		})
	}
}

// media.serve 只是告警：图片仍能访问，只是绕过了 Nginx。
func TestWarningsMediaServe(t *testing.T) {
	dev := defaults()
	if got := dev.Warnings(); got != nil {
		t.Errorf("开发环境不该有告警，实际 %v", got)
	}

	prod := prodConfig()
	if got := prod.Warnings(); got != nil {
		t.Errorf("media.serve 为 false 时不该有告警，实际 %v", got)
	}
	prod.Media.Serve = true
	got := prod.Warnings()
	if len(got) != 1 || !strings.Contains(got[0], "media.serve") {
		t.Errorf("应提示 media.serve，实际 %v", got)
	}
}
