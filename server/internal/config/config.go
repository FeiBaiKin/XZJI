// Package config 加载 YAML 配置，敏感项可用环境变量覆盖。
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Env  string `yaml:"env"`
	HTTP HTTP   `yaml:"http"`
	DB   DB     `yaml:"db"`
	Log  Log    `yaml:"log"`
	CORS CORS   `yaml:"cors"`
	Auth Auth   `yaml:"auth"`
	Mail Mail   `yaml:"mail"`
}

type HTTP struct {
	Addr string `yaml:"addr"`
	// TrustedProxies 只信任这些地址传来的 X-Real-IP，通常是本机 Nginx。
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type DB struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	AutoMigrate     bool          `yaml:"auto_migrate"`
}

type Log struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type CORS struct {
	// AllowOrigins 允许跨域访问前台接口的前台域名，必须精确匹配。
	AllowOrigins []string `yaml:"allow_origins"`
}

type Auth struct {
	// CookieDomain 前台登录 Cookie 的作用域，生产填 .example.com 让 www 与 api 共享；留空则只作用于接口域名
	CookieDomain string `yaml:"cookie_domain"`
	// CookieSecure Cookie 只在 HTTPS 下发送，生产必须为 true
	CookieSecure bool `yaml:"cookie_secure"`
}

type Mail struct {
	// Driver 为 log 时只把邮件内容写进日志（开发用），smtp 为真实发送
	Driver string `yaml:"driver"`
	SMTP   SMTP   `yaml:"smtp"`
}

type SMTP struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	// ImplicitTLS 465 端口为 true；587 端口为 false，使用 STARTTLS
	ImplicitTLS bool `yaml:"implicit_tls"`
}

const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

func (c *Config) IsProd() bool { return c.Env == EnvProd }

// Load 读取配置文件，未知字段视为错误，避免拼错配置项而不自知。
func Load(path string) (*Config, error) {
	cfg := defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s：%w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s：%w", path, err)
	}
	applyEnv(cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Env:  EnvDev,
		HTTP: HTTP{Addr: "127.0.0.1:8080", TrustedProxies: []string{"127.0.0.1", "::1"}},
		DB:   DB{MaxOpenConns: 20, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute, AutoMigrate: true},
		Log:  Log{Level: "info", Format: "text"},
		Mail: Mail{Driver: "log", SMTP: SMTP{Port: 465, ImplicitTLS: true}},
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("XZJI_DB_DSN"); v != "" {
		cfg.DB.DSN = v
	}
	if v := os.Getenv("XZJI_HTTP_ADDR"); v != "" {
		cfg.HTTP.Addr = v
	}
	if v := os.Getenv("XZJI_SMTP_PASSWORD"); v != "" {
		cfg.Mail.SMTP.Password = v
	}
}

func (c *Config) validate() error {
	var errs []error
	if c.Env != EnvDev && c.Env != EnvProd {
		errs = append(errs, fmt.Errorf("env 只能是 %s 或 %s", EnvDev, EnvProd))
	}
	if c.DB.DSN == "" {
		errs = append(errs, errors.New("db.dsn 不能为空"))
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		errs = append(errs, errors.New("log.format 只能是 text 或 json"))
	}
	switch c.Mail.Driver {
	case "log":
	case "smtp":
		if c.Mail.SMTP.Host == "" || c.Mail.SMTP.From == "" {
			errs = append(errs, errors.New("mail.driver 为 smtp 时，mail.smtp.host 和 mail.smtp.from 不能为空"))
		}
	default:
		errs = append(errs, errors.New("mail.driver 只能是 log 或 smtp"))
	}
	if c.IsProd() {
		if !c.Auth.CookieSecure {
			errs = append(errs, errors.New("生产环境必须开启 auth.cookie_secure"))
		}
		if c.Mail.Driver != "smtp" {
			errs = append(errs, errors.New("生产环境必须配置 SMTP 发信（mail.driver: smtp），否则用户收不到验证码"))
		}
	}
	return errors.Join(errs...)
}
