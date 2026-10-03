// Package setting 读写站点设置，值以 JSON 存储。
package setting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"xzji/internal/dbgen"
)

const KeySiteName = "site.name"

const defaultSiteName = "XZJI"

var ErrNotFound = errors.New("设置项不存在")

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

// Get 读取设置项并解码到 dst。
func (s *Service) Get(ctx context.Context, key string, dst any) error {
	raw, err := s.q.GetSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("读取设置 %s：%w", key, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("解析设置 %s：%w", key, err)
	}
	return nil
}

// Set 写入设置项，已存在则覆盖。
func (s *Service) Set(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("编码设置 %s：%w", key, err)
	}
	return s.q.UpsertSetting(ctx, dbgen.UpsertSettingParams{Key: key, Value: raw})
}

// SiteName 返回站点名称，未设置时返回默认值。
func (s *Service) SiteName(ctx context.Context) (string, error) {
	var name string
	err := s.Get(ctx, KeySiteName, &name)
	if errors.Is(err, ErrNotFound) {
		return defaultSiteName, nil
	}
	return name, err
}
