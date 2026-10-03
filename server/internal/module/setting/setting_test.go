package setting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"xzji/internal/dbgen"
)

type fakeQuerier struct {
	data map[string]json.RawMessage
}

func (f *fakeQuerier) GetSetting(_ context.Context, key string) (json.RawMessage, error) {
	v, ok := f.data[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}

func (f *fakeQuerier) UpsertSetting(_ context.Context, arg dbgen.UpsertSettingParams) error {
	f.data[arg.Key] = arg.Value
	return nil
}

func TestSiteName(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]json.RawMessage
		want    string
		wantErr bool
	}{
		{name: "未设置时返回默认值", data: map[string]json.RawMessage{}, want: defaultSiteName},
		{name: "读取已保存的值", data: map[string]json.RawMessage{KeySiteName: json.RawMessage(`"小站集"`)}, want: "小站集"},
		{name: "值不是字符串时报错", data: map[string]json.RawMessage{KeySiteName: json.RawMessage(`123`)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService(&fakeQuerier{data: tt.data})
			got, err := s.SiteName(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetThenGet(t *testing.T) {
	s := NewService(&fakeQuerier{data: map[string]json.RawMessage{}})
	ctx := context.Background()
	if err := s.Set(ctx, "pay.order_ttl_minutes", 30); err != nil {
		t.Fatal(err)
	}
	var ttl int
	if err := s.Get(ctx, "pay.order_ttl_minutes", &ttl); err != nil {
		t.Fatal(err)
	}
	if ttl != 30 {
		t.Fatalf("got %d, want 30", ttl)
	}
	if err := s.Get(ctx, "missing", &ttl); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
