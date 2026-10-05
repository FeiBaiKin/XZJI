package setting

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"xzji/internal/platform/errs"
)

// errCode 取出业务错误码，没有错误时返回空串。
func errCode(err error) string {
	var de *errs.Error
	if errors.As(err, &de) {
		return de.Code
	}
	if err == nil {
		return ""
	}
	return "非业务错误"
}

// fakeSanitizer 记录收到的原文，返回一个能被识别的结果，用来验证保存走的是过滤后的内容。
type fakeSanitizer struct {
	got     []string
	counter int
}

func (f *fakeSanitizer) SanitizeBody(raw string) (string, error) {
	f.got = append(f.got, raw)
	f.counter++
	return "<p>过滤后的正文</p>", nil
}

func (f *fakeSanitizer) RenderBody(stored string) string {
	return strings.ReplaceAll(stored, "@media/", "https://img.example.com/")
}

func newSiteService(q *fakeQuerier) (*SiteService, *fakeSanitizer) {
	s := &fakeSanitizer{}
	return NewSiteService(NewService(q), s), s
}

func TestSiteSet(t *testing.T) {
	long := strings.Repeat("字", maxDocLen+1)
	cases := []struct {
		name string
		in   Site
		code string
	}{
		{"正常保存", Site{FooterAbout: "整理安卓和电脑软件。", ICP: "京ICP备00000000号-1", PoliceICP: "京公网安备11010502000000号", TermsHTML: "<p>协议</p>"}, ""},
		{"协议可以留空", Site{FooterAbout: "简介", TermsHTML: "", PrivacyHTML: ""}, ""},
		{"页脚简介超长", Site{FooterAbout: strings.Repeat("字", 201)}, "FIELD_TOO_LONG"},
		{"备案号超长", Site{ICP: strings.Repeat("字", 61)}, "FIELD_TOO_LONG"},
		{"协议正文超长", Site{TermsHTML: long}, "FIELD_TOO_LONG"},
		{"隐私政策正文超长", Site{PrivacyHTML: long}, "FIELD_TOO_LONG"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newSiteService(&fakeQuerier{data: map[string]json.RawMessage{}})
			_, err := svc.Set(context.Background(), c.in)
			if got := errCode(err); got != c.code {
				t.Fatalf("错误码 = %q，期望 %q（err = %v）", got, c.code, err)
			}
		})
	}
}

// TestSiteSetFiltersBody 正文先过滤再入库，读取时再还原图片地址。
func TestSiteSetFiltersBody(t *testing.T) {
	q := &fakeQuerier{data: map[string]json.RawMessage{}}
	svc, sanitizer := newSiteService(q)
	ctx := context.Background()

	saved, err := svc.Set(ctx, Site{FooterAbout: "简介", TermsHTML: "<p>原始协议<script>alert(1)</script></p>"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sanitizer.got) != 1 || !strings.Contains(sanitizer.got[0], "script") {
		t.Fatalf("净化器应收到原始正文，实际 %q", sanitizer.got)
	}
	// 入库的是过滤后的内容，读出来也一样；图片占位符在读取时还原
	var stored Site
	if err := json.Unmarshal(q.data[KeySite], &stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.TermsHTML, "script") {
		t.Fatalf("入库的正文没有经过过滤：%q", stored.TermsHTML)
	}
	if saved.TermsHTML != "<p>过滤后的正文</p>" {
		t.Fatalf("正文 = %q，期望过滤后的内容", saved.TermsHTML)
	}

	// 留空时不调用净化器
	if _, err := svc.Set(ctx, Site{PrivacyHTML: "  "}); err != nil {
		t.Fatal(err)
	}
	if len(sanitizer.got) != 1 {
		t.Fatalf("空正文不该调用净化器，实际调用 %d 次", len(sanitizer.got))
	}
}

func TestSiteGetMissing(t *testing.T) {
	svc, _ := newSiteService(&fakeQuerier{data: map[string]json.RawMessage{}})
	site, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if site != (Site{}) {
		t.Fatalf("没设置过时应返回空值，实际 %+v", site)
	}
}

// TestSiteGetRendersMedia 入库时正文里的图片地址是占位符，读取时换回当前地址。
func TestSiteGetRendersMedia(t *testing.T) {
	q := &fakeQuerier{data: map[string]json.RawMessage{
		KeySite: json.RawMessage(`{"footerAbout":"简介","termsHtml":"<p><img src=\"@media/a.webp\"></p>","privacyHtml":""}`),
	}}
	svc, _ := newSiteService(q)
	site, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := `<img src="https://img.example.com/a.webp">`; !strings.Contains(site.TermsHTML, want) {
		t.Fatalf("正文 = %q，期望包含 %q", site.TermsHTML, want)
	}
}
