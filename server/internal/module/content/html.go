package content

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// mediaToken 正文里本站图片地址的前缀。入库时把 media.base_url 换成它，输出时再换回来，
// 这样更换图片域名或切换到 CDN 时不需要改数据。
const mediaToken = "@media/"

const autoSummaryLen = 120

var policy = newPolicy()

// newPolicy 正文白名单：只保留编辑器能产生的排版标签，去掉脚本、样式和事件属性。
func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowStandardURLs()
	p.RequireParseableURLs(true)
	p.AllowElements(
		"p", "br", "hr", "h2", "h3", "h4", "strong", "b", "em", "i", "u", "s", "del", "mark", "sub", "sup",
		"blockquote", "ul", "ol", "li", "pre", "code", "table", "thead", "tbody", "tr", "th", "td", "figure", "figcaption",
	)
	p.AllowAttrs("href").OnElements("a")
	p.RequireNoFollowOnFullyQualifiedLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	p.AllowAttrs("width", "height").Matching(bluemonday.Integer).OnElements("img")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[A-Za-z0-9+#_-]{1,32}$`)).OnElements("code")
	p.AllowAttrs("colspan", "rowspan").Matching(bluemonday.Integer).OnElements("td", "th")
	p.AllowAttrs("start").Matching(bluemonday.Integer).OnElements("ol")
	// 隐藏块的占位符，内容另存，见 splitHidden
	p.AllowAttrs("data-hidden-block").Matching(regexp.MustCompile(`^\d{1,2}$`)).OnElements("div")
	return p
}

type processed struct {
	HTML string
	TOC  []TOCItem
	Text string
}

// processHTML 过滤编辑器提交的 HTML，给二、三级标题加锚点（id 以 idPrefix 开头）并提取目录，
// 把本站图片地址换成 mediaToken，同时提取段落文字用于生成摘要。
func processHTML(raw, mediaBase, idPrefix string) (processed, error) {
	clean := policy.Sanitize(raw)
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(clean), body)
	if err != nil {
		return processed{}, fmt.Errorf("解析正文：%w", err)
	}

	var out processed
	var text strings.Builder
	prefix := mediaBase + "/"
	// skipText 为 true 时不计入摘要：标题、代码和表格截出来读不通
	var walk func(n *html.Node, skipText bool)
	walk = func(n *html.Node, skipText bool) {
		switch n.DataAtom {
		case atom.H2, atom.H3, atom.H4, atom.Pre, atom.Table, atom.Figcaption:
			skipText = true
		}
		switch {
		case n.Type == html.TextNode:
			if !skipText {
				text.WriteString(n.Data)
				text.WriteByte(' ')
			}
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3:
			id := fmt.Sprintf("%s%d", idPrefix, len(out.TOC)+1)
			setAttr(n, "id", id)
			if t := truncate(collapseSpace(nodeText(n)), 80); t != "" {
				level := 2
				if n.DataAtom == atom.H3 {
					level = 3
				}
				out.TOC = append(out.TOC, TOCItem{ID: id, Text: t, Level: level})
			}
		case n.DataAtom == atom.Img:
			if src := getAttr(n, "src"); strings.HasPrefix(src, prefix) {
				setAttr(n, "src", mediaToken+strings.TrimPrefix(src, prefix))
			}
			setAttr(n, "loading", "lazy")
			setAttr(n, "decoding", "async")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skipText)
		}
	}

	var buf strings.Builder
	for _, n := range nodes {
		walk(n, false)
		if err := html.Render(&buf, n); err != nil {
			return processed{}, fmt.Errorf("生成正文：%w", err)
		}
	}
	out.HTML = buf.String()
	out.Text = collapseSpace(text.String())
	return out, nil
}

// outputHTML 把入库正文中的 mediaToken 换回当前的图片地址。
func outputHTML(stored, mediaBase string) string {
	return strings.ReplaceAll(stored, `src="`+mediaToken, `src="`+mediaBase+"/")
}

func autoSummary(text string) string {
	return truncate(text, autoSummaryLen)
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate 按字符截断，超出时末尾加省略号。
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}

const hiddenType = "hidden-block"

// hiddenPlaceholder 正文中隐藏块的占位符，前台据此在原位置显示解锁框或解锁后的内容。
var hiddenPlaceholder = regexp.MustCompile(`<div data-hidden-block="(\d+)"></div>`)

// rawHidden 编辑器提交的隐藏块，内容尚未过滤。
type rawHidden struct {
	Cond  string
	Title string
	Inner string
}

func isHidden(n *html.Node) bool {
	return n.Type == html.ElementNode && n.DataAtom == atom.Div && getAttr(n, "data-type") == hiddenType
}

func containsHidden(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isHidden(c) || containsHidden(c) {
			return true
		}
	}
	return false
}

// splitHidden 把顶层的隐藏块（<div data-type="hidden-block" data-cond="…" data-title="…">）从正文中拆出来，
// 原位置换成占位符。隐藏块只能放在顶层：嵌在列表或引用里的会在过滤时丢掉外层标记，
// 内容变成公开的，所以直接拒绝保存。
func splitHidden(raw string) (string, []rawHidden, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(raw), body)
	if err != nil {
		return "", nil, fmt.Errorf("解析正文：%w", err)
	}
	var blocks []rawHidden
	var buf strings.Builder
	for _, n := range nodes {
		if !isHidden(n) {
			if containsHidden(n) {
				return "", nil, ErrNestedHidden
			}
			if err := html.Render(&buf, n); err != nil {
				return "", nil, fmt.Errorf("生成正文：%w", err)
			}
			continue
		}
		if containsHidden(n) {
			return "", nil, ErrNestedHidden
		}
		var inner strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := html.Render(&inner, c); err != nil {
				return "", nil, fmt.Errorf("生成隐藏内容：%w", err)
			}
		}
		blocks = append(blocks, rawHidden{Cond: getAttr(n, "data-cond"), Title: getAttr(n, "data-title"), Inner: inner.String()})
		fmt.Fprintf(&buf, `<div data-hidden-block="%d"></div>`, len(blocks))
	}
	return buf.String(), blocks, nil
}

// mergeHidden 后台编辑时把隐藏块放回占位符的位置，还原成编辑器的格式。
func mergeHidden(public string, blocks []HiddenBlock) string {
	bySeq := make(map[string]HiddenBlock, len(blocks))
	for _, b := range blocks {
		bySeq[strconv.Itoa(b.Seq)] = b
	}
	return hiddenPlaceholder.ReplaceAllStringFunc(public, func(m string) string {
		b, ok := bySeq[hiddenPlaceholder.FindStringSubmatch(m)[1]]
		if !ok {
			return ""
		}
		return fmt.Sprintf(`<div data-type="%s" data-cond="%s" data-title="%s">%s</div>`,
			hiddenType, html.EscapeString(b.Cond), html.EscapeString(b.Title), b.ContentHTML)
	})
}
