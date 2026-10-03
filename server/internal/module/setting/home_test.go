package setting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"xzji/internal/module/media"
	"xzji/internal/platform/errs"
)

type fakeMedia map[int64]media.Media

func (f fakeMedia) GetMany(_ context.Context, ids []int64) (map[int64]media.Media, error) {
	out := map[int64]media.Media{}
	for _, id := range ids {
		if m, ok := f[id]; ok {
			out[id] = m
		}
	}
	return out, nil
}

func TestHomeSet(t *testing.T) {
	ok := Banner{Title: "装机必备", Link: "/category/pc/"}
	cases := []struct {
		name string
		in   Home
		code string
	}{
		{"正常保存", Home{Banners: []Banner{ok, {Title: "外链", Link: "https://example.com", ImageID: 1}}, Notice: "国庆活动"}, ""},
		{"标题必填", Home{Banners: []Banner{{Title: " ", Link: "/"}}}, "FIELD_REQUIRED"},
		{"拒绝 javascript 链接", Home{Banners: []Banner{{Title: "x", Link: "javascript:alert(1)"}}}, "INVALID_LINK"},
		{"拒绝协议相对地址", Home{Banners: []Banner{{Title: "x", Link: "//evil.example"}}}, "INVALID_LINK"},
		{"背景图不存在", Home{Banners: []Banner{{Title: "x", Link: "/", ImageID: 9}}}, "MEDIA_NOT_FOUND"},
		{"最多 5 张", Home{Banners: []Banner{ok, ok, ok, ok, ok, ok}}, "TOO_MANY_BANNERS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := NewHomeService(NewService(&fakeQuerier{data: map[string]json.RawMessage{}}), fakeMedia{1: {ID: 1, URL: "https://img.example.com/a.png"}})
			got, err := h.Set(context.Background(), c.in)
			if c.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				if len(got.Banners) != 2 || got.Banners[1].Image == nil || got.Notice != "国庆活动" {
					t.Fatalf("保存结果不对：%+v", got)
				}
				return
			}
			var de *errs.Error
			if !errors.As(err, &de) || de.Code != c.code {
				t.Fatalf("期望 %s，得到 %v", c.code, err)
			}
		})
	}
}

func TestHomeGetDefault(t *testing.T) {
	h := NewHomeService(NewService(&fakeQuerier{data: map[string]json.RawMessage{}}), fakeMedia{})
	got, err := h.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Banners == nil || len(got.Banners) != 0 || got.Notice != "" {
		t.Fatalf("未设置时应返回空配置：%+v", got)
	}
}
