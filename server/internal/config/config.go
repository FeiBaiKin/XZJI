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
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("XZJI_DB_DSN"); v != "" {
		cfg.DB.DSN = v
	}
	if v := os.Getenv("XZJI_HTTP_ADDR"); v != "" {
		cfg.HTTP.Addr = v
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
	return errors.Join(errs...)
}
