package setting

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"xzji/internal/module/media"
	"xzji/internal/platform/errs"
)

const (
	KeyHome    = "home"
	maxBanners = 5
)

type Banner struct {
	Tag        string `json:"tag"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	ButtonText string `json:"buttonText"`
	Link       string `json:"link"`
	ImageID    int64  `json:"imageId,omitempty"`
	// Image 读取时由 ImageID 解析，不入库
	Image *media.Media `json:"-"`
}

// Home 首页运营位：横幅轮播和公告。
type Home struct {
	Banners []Banner `json:"banners"`
	Notice  string   `json:"notice"`
}

// MediaLookup 解析横幅背景图。
type MediaLookup interface {
	GetMany(ctx context.Context, ids []int64) (map[int64]media.Media, error)
}

type HomeService struct {
	settings *Service
	media    MediaLookup
}

func NewHomeService(settings *Service, m MediaLookup) *HomeService {
	return &HomeService{settings: settings, media: m}
}

// Get 读取首页设置，未设置时返回空配置。背景图已被删除的横幅照常返回，只是没有图。
func (h *HomeService) Get(ctx context.Context) (Home, error) {
	home := Home{Banners: []Banner{}}
	if err := h.settings.Get(ctx, KeyHome, &home); err != nil && !errors.Is(err, ErrNotFound) {
		return Home{}, err
	}
	if home.Banners == nil {
		home.Banners = []Banner{}
	}
	ids := make([]int64, len(home.Banners))
	for i, b := range home.Banners {
		ids[i] = b.ImageID
	}
	found, err := h.media.GetMany(ctx, ids)
	if err != nil {
		return Home{}, err
	}
	for i := range home.Banners {
		if m, ok := found[home.Banners[i].ImageID]; ok {
			home.Banners[i].Image = &m
		}
	}
	return home, nil
}

func field(name, s string, max int, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", errs.Invalid("FIELD_REQUIRED", "请填写"+name)
	}
	if utf8.RuneCountInString(s) > max {
		return "", errs.Invalid("FIELD_TOO_LONG", name+"太长了")
	}
	return s, nil
}

// validLink 只允许站内路径和 http(s) 网址，防止 javascript: 之类的链接。
func validLink(s string) bool {
	if strings.HasPrefix(s, "/") {
		return !strings.HasPrefix(s, "//")
	}
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")
}

func (h *HomeService) Set(ctx context.Context, in Home) (Home, error) {
	if len(in.Banners) > maxBanners {
		return Home{}, errs.Invalid("TOO_MANY_BANNERS", "横幅最多 5 张")
	}
	out := Home{Banners: make([]Banner, 0, len(in.Banners))}
	ids := []int64{}
	for _, b := range in.Banners {
		var err error
		if b.Title, err = field("横幅标题", b.Title, 60, true); err != nil {
			return Home{}, err
		}
		if b.Tag, err = field("横幅标签", b.Tag, 12, false); err != nil {
			return Home{}, err
		}
		if b.Subtitle, err = field("横幅说明", b.Subtitle, 120, false); err != nil {
			return Home{}, err
		}
		if b.ButtonText, err = field("按钮文字", b.ButtonText, 12, false); err != nil {
			return Home{}, err
		}
		if b.Link, err = field("横幅链接", b.Link, 500, true); err != nil {
			return Home{}, err
		}
		if !validLink(b.Link) {
			return Home{}, errs.Invalid("INVALID_LINK", "链接需要以 / 开头，或者是 http(s) 网址")
		}
		if b.ImageID < 0 {
			b.ImageID = 0
		}
		ids = append(ids, b.ImageID)
		b.Image = nil
		out.Banners = append(out.Banners, b)
	}
	var err error
	if out.Notice, err = field("公告", in.Notice, 1000, false); err != nil {
		return Home{}, err
	}
	found, err := h.media.GetMany(ctx, ids)
	if err != nil {
		return Home{}, err
	}
	for _, id := range ids {
		if _, ok := found[id]; id > 0 && !ok {
			return Home{}, media.ErrNotFound
		}
	}
	if err := h.settings.Set(ctx, KeyHome, out); err != nil {
		return Home{}, err
	}
	return h.Get(ctx)
}
