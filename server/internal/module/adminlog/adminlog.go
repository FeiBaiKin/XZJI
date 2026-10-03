// Package adminlog 记录后台敏感操作，便于事后追查。
package adminlog

import (
	"context"
	"encoding/json"
	"fmt"

	"xzji/internal/dbgen"
)

type Service struct {
	q dbgen.Querier
}

func NewService(q dbgen.Querier) *Service { return &Service{q: q} }

// Record 写入一条后台操作日志。adminID 为 0 表示命令行操作。
func (s *Service) Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error {
	var raw json.RawMessage
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("编码操作日志详情：%w", err)
		}
		raw = b
	}
	if err := s.q.CreateAdminLog(ctx, dbgen.CreateAdminLogParams{
		AdminID: adminID, Action: action, TargetType: targetType, TargetID: targetID, Detail: raw, Ip: ip,
	}); err != nil {
		return fmt.Errorf("写入后台操作日志：%w", err)
	}
	return nil
}
