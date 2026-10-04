package content

import (
	"strings"
	"testing"
)

const base = "https://img.example.com"

func TestProcessHTMLSanitize(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []string
		notWant []string
	}{
		{
			name:    "去掉脚本和事件属性",
			in:      `<p onclick="alert(1)">你好<script>alert(1)</script></p><img src="x" onerror="alert(1)">`,
			want:    []string{"<p>你好</p>"},
			notWant: []string{"script", "onclick", "onerror", "alert"},
		},
		{
			name:    "javascript 链接被移除",
			in:      `<p><a href="javascript:alert(1)">点我</a></p>`,
			want:    []string{"点我"},
			notWant: []string{"javascript", "<a"},
		},
		{
			name: "外链加 nofollow 并新窗口打开",
			in:   `<p><a href="https://example.com">官网</a></p>`,
			want: []string{`href="https://example.com"`, `nofollow`, `target="_blank"`},
		},
		{
			name:    "保留代码语言但去掉其他 class 和样式",
			in:      `<pre><code class="language-nginx">gzip on;</code></pre><p class="x" style="color:red">文字</p>`,
			want:    []string{`<code class="language-nginx">`, "<p>文字</p>"},
			notWant: []string{"style", `class="x"`},
		},
		{
			name:    "不允许 iframe 和一级标题标签",
			in:      `<h1>大标题</h1><iframe src="https://evil.example"></iframe>`,
			want:    []string{"大标题"},
			notWant: []string{"<h1", "iframe"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := processHTML(c.in, base, "h-")
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !strings.Contains(got.HTML, w) {
					t.Errorf("结果缺少 %q：%s", w, got.HTML)
				}
			}
			for _, w := range c.notWant {
				if strings.Contains(got.HTML, w) {
					t.Errorf("结果不应包含 %q：%s", w, got.HTML)
				}
			}
		})
	}
}

func TestProcessHTMLTOCAndMedia(t *testing.T) {
	in := `<h2>资源 <em>介绍</em></h2><p>第一段</p><h3>便携版</h3><h4>不进目录</h4>` +
		`<p><img src="https://img.example.com/2026/10/a.png" alt="截图"></p>` +
		`<p><img src="https://other.example.com/b.png"></p>`
	got, err := processHTML(in, base, "h-")
	if err != nil {
		t.Fatal(err)
	}
	wantTOC := []TOCItem{{ID: "h-1", Text: "资源 介绍", Level: 2}, {ID: "h-2", Text: "便携版", Level: 3}}
	if len(got.TOC) != len(wantTOC) {
		t.Fatalf("目录 = %+v", got.TOC)
	}
	for i := range wantTOC {
		if got.TOC[i] != wantTOC[i] {
			t.Fatalf("目录第 %d 项 = %+v，期望 %+v", i, got.TOC[i], wantTOC[i])
		}
	}
	for _, w := range []string{`<h2 id="h-1">`, `<h3 id="h-2">`, `src="@media/2026/10/a.png"`, `src="https://other.example.com/b.png"`, `loading="lazy"`} {
		if !strings.Contains(got.HTML, w) {
			t.Errorf("结果缺少 %q：%s", w, got.HTML)
		}
	}
	out := outputHTML(got.HTML, "https://cdn.example.com")
	if !strings.Contains(out, `src="https://cdn.example.com/2026/10/a.png"`) {
		t.Errorf("输出时没有换成新的图片地址：%s", out)
	}
	if got.Text != "第一段" {
		t.Errorf("纯文本 = %q", got.Text)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"短文本", 10, "短文本"},
		{"一二三四五六", 3, "一二三…"},
		{"ab cd", 3, "ab…"},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q，期望 %q", c.in, c.n, got, c.want)
		}
	}
}

func TestSplitHidden(t *testing.T) {
	in := `<p>公开段落</p>` +
		`<div data-type="hidden-block" data-cond="paid" data-title="语法速查"><p>付费内容</p><h2>隐藏标题</h2></div>` +
		`<p>中间</p>` +
		`<div data-type="hidden-block" data-cond="login" data-title=""><p>登录内容</p></div>`
	public, blocks, err := splitHidden(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Cond != "paid" || blocks[0].Title != "语法速查" || blocks[1].Cond != "login" {
		t.Fatalf("隐藏块 = %+v", blocks)
	}
	if strings.Contains(public, "付费内容") || strings.Contains(public, "登录内容") {
		t.Fatalf("公开正文里出现了隐藏内容：%s", public)
	}
	got, err := processHTML(public, base, "h-")
	if err != nil {
		t.Fatal(err)
	}
	want := `<p>公开段落</p><div data-hidden-block="1"></div><p>中间</p><div data-hidden-block="2"></div>`
	if got.HTML != want {
		t.Fatalf("公开正文 = %s，期望 %s", got.HTML, want)
	}
	if len(got.TOC) != 0 {
		t.Fatalf("隐藏块里的标题不应进目录：%+v", got.TOC)
	}

	merged := mergeHidden(got.HTML, []HiddenBlock{{Seq: 1, Cond: "paid", Title: `语法"速查`, ContentHTML: "<p>付费内容</p>"}})
	for _, w := range []string{`<div data-type="hidden-block" data-cond="paid" data-title="语法&#34;速查"><p>付费内容</p></div>`, `<p>中间</p>`} {
		if !strings.Contains(merged, w) {
			t.Errorf("还原结果缺少 %q：%s", w, merged)
		}
	}
	if strings.Contains(merged, `data-hidden-block="2"`) {
		t.Errorf("找不到内容的占位符应当去掉：%s", merged)
	}
}

func TestSplitHiddenRejectsNested(t *testing.T) {
	cases := []string{
		`<ul><li><div data-type="hidden-block" data-cond="login"><p>x</p></div></li></ul>`,
		`<blockquote><div data-type="hidden-block" data-cond="login"><p>x</p></div></blockquote>`,
		`<div data-type="hidden-block" data-cond="login"><div data-type="hidden-block" data-cond="paid"><p>x</p></div></div>`,
	}
	for _, in := range cases {
		if _, _, err := splitHidden(in); err != ErrNestedHidden {
			t.Errorf("splitHidden(%s) 应当拒绝，得到 %v", in, err)
		}
	}
}
