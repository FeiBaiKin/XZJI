package setting

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"xzji/internal/platform/errs"
)

const (
	KeySite = "site"
	// maxDocLen 协议正文的长度上限（字符数），在过滤前判断，避免为一个超大文档做白名单扫描
	maxDocLen = 100000
)

// Site 站点信息：页脚内容和两份法律文档。
type Site struct {
	FooterAbout string `json:"footerAbout"`
	ICP         string `json:"icp"`
	PoliceICP   string `json:"policeIcp"`
	// TermsHTML 和 PrivacyHTML 入库前已过滤，输出时把图片占位符换回当前图片地址
	TermsHTML   string `json:"termsHtml"`
	PrivacyHTML string `json:"privacyHtml"`
}

// Sanitizer 过滤富文本，由 content 模块实现，保证协议正文和内容正文用同一套白名单。
type Sanitizer interface {
	// SanitizeBody 过滤编辑器提交的 HTML，返回可以入库的正文
	SanitizeBody(raw string) (string, error)
	// RenderBody 把入库的正文还原成可以输出的 HTML
	RenderBody(stored string) string
}

type SiteService struct {
	settings  *Service
	sanitizer Sanitizer
}

func NewSiteService(settings *Service, sanitizer Sanitizer) *SiteService {
	return &SiteService{settings: settings, sanitizer: sanitizer}
}

// Get 读取站点设置。没设置过时返回空值：页脚只显示站点名称，协议页提示站长尚未填写。
func (s *SiteService) Get(ctx context.Context) (Site, error) {
	site := Site{}
	if err := s.settings.Get(ctx, KeySite, &site); err != nil && !errors.Is(err, ErrNotFound) {
		return Site{}, err
	}
	site.TermsHTML = s.sanitizer.RenderBody(site.TermsHTML)
	site.PrivacyHTML = s.sanitizer.RenderBody(site.PrivacyHTML)
	return site, nil
}

// Set 校验并保存站点设置。
func (s *SiteService) Set(ctx context.Context, in Site) (Site, error) {
	var err error
	if in.FooterAbout, err = field("页脚简介", in.FooterAbout, 200, false); err != nil {
		return Site{}, err
	}
	if in.ICP, err = field("ICP 备案号", in.ICP, 60, false); err != nil {
		return Site{}, err
	}
	if in.PoliceICP, err = field("公安备案号", in.PoliceICP, 60, false); err != nil {
		return Site{}, err
	}
	if in.TermsHTML, err = s.doc("用户协议", in.TermsHTML); err != nil {
		return Site{}, err
	}
	if in.PrivacyHTML, err = s.doc("隐私政策", in.PrivacyHTML); err != nil {
		return Site{}, err
	}
	if err := s.settings.Set(ctx, KeySite, in); err != nil {
		return Site{}, err
	}
	return s.Get(ctx)
}

func (s *SiteService) doc(name, raw string) (string, error) {
	if utf8.RuneCountInString(raw) > maxDocLen {
		return "", errs.Invalid("FIELD_TOO_LONG", name+"正文不能超过 "+strconv.Itoa(maxDocLen)+" 个字")
	}
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	out, err := s.sanitizer.SanitizeBody(raw)
	if err != nil {
		return "", fmt.Errorf("过滤%s正文：%w", name, err)
	}
	return out, nil
}
