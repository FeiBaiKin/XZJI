// Package content 管理分类、标签和内容（文章与资源）。
package content

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"xzji/internal/dbgen"
	"xzji/internal/module/media"
	"xzji/internal/platform/errs"
)

type Type string

const (
	TypeArticle  Type = "article"
	TypeResource Type = "resource"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusTrash     Status = "trash"
)

type Category struct {
	ID             int64
	ParentID       *int64
	Name           string
	Slug           string
	Icon           string
	Color          string
	Description    string
	Sort           int
	SEOTitle       string
	SEODescription string
	// PostCount 树形结果里包含子分类，平铺结果里只算自身
	PostCount int64
	Children  []*Category
}

type Tag struct {
	ID        int64
	Name      string
	Slug      string
	PostCount int64
	CreatedAt time.Time
}

// Ref 分类或标签的简要信息。
type Ref struct {
	ID   int64
	Name string
	Slug string
}

type Author struct {
	ID       int64
	Nickname string
}

type TOCItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Level int    `json:"level"`
}

type Attr struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ChangelogEntry struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Note    string `json:"note"`
}

type Resource struct {
	Platform  string
	Version   string
	FileSize  string
	Icon      *media.Media
	Attrs     []Attr
	Changelog []ChangelogEntry
}

type Post struct {
	ID      int64
	Type    Type
	Title   string
	Summary string
	// SummaryAuto 摘要由正文自动截取，只在查询单篇时填充
	SummaryAuto bool
	// ContentHTML 和 TOC 只在查询单篇时填充
	ContentHTML    string
	TOC            []TOCItem
	Cover          *media.Media
	Category       Ref
	Tags           []Ref
	Author         Author
	Status         Status
	Pinned         bool
	ViewCount      int
	CommentCount   int
	SEOTitle       string
	SEOKeywords    string
	SEODescription string
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Resource       *Resource
}

var (
	ErrCategoryNotFound = errs.NotFound("CATEGORY_NOT_FOUND", "分类不存在")
	ErrTagNotFound      = errs.NotFound("TAG_NOT_FOUND", "标签不存在")
	ErrPostNotFound     = errs.NotFound("POST_NOT_FOUND", "内容不存在或已下线")
	ErrSlugTaken        = errs.New(errs.KindConflict, "SLUG_TAKEN", "这个 slug 已经被使用了")
	ErrTagNameTaken     = errs.New(errs.KindConflict, "TAG_NAME_TAKEN", "已有同名标签")
	ErrCategoryInUse    = errs.New(errs.KindConflict, "CATEGORY_IN_USE", "分类下还有子分类或内容，不能删除")
	ErrInvalidParent    = errs.Invalid("INVALID_PARENT", "只支持两级分类，上级分类必须是顶级分类")
	ErrInvalidSlug      = errs.Invalid("INVALID_SLUG", "slug 只能包含小写字母、数字和短横线")
	ErrInvalidTagSlug   = errs.Invalid("INVALID_SLUG", "slug 只能包含文字、数字和短横线")
	ErrInvalidColor     = errs.Invalid("INVALID_COLOR", "颜色不正确")
	ErrInvalidIcon      = errs.Invalid("INVALID_ICON", "图标名称不正确")
	ErrInvalidType      = errs.Invalid("INVALID_TYPE", "内容类型不正确")
	ErrInvalidStatus    = errs.Invalid("INVALID_STATUS", "内容状态不正确")
	ErrTitleRequired    = errs.Invalid("TITLE_REQUIRED", "请填写标题")
	ErrNameRequired     = errs.Invalid("NAME_REQUIRED", "请填写名称")
	ErrTooManyTags      = errs.Invalid("TOO_MANY_TAGS", "标签最多 10 个")
	ErrInvalidDate      = errs.Invalid("INVALID_DATE", "日期格式应为 YYYY-MM-DD")
	ErrNotInTrash       = errs.Invalid("NOT_IN_TRASH", "只能彻底删除回收站里的内容")
	ErrInTrash          = errs.Invalid("POST_IN_TRASH", "回收站里的内容需要先恢复才能编辑")
)

func tooLong(field string, n int) *errs.Error {
	return errs.Invalid("FIELD_TOO_LONG", field+"不能超过 "+strconv.Itoa(n)+" 个字")
}

// checkLen 去掉首尾空白后检查长度。
func checkLen(field, s string, n int) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > n {
		return "", tooLong(field, n)
	}
	return s, nil
}

var (
	slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	iconRe = regexp.MustCompile(`^[a-z0-9-]{0,32}$`)
)

var colors = map[string]bool{"": true, "blue": true, "red": true, "orange": true, "green": true, "purple": true, "gold": true, "vip": true}

// tagSlugOK 标签 slug 允许中文，便于直接用标签名做网址。
func tagSlugOK(s string) bool {
	if s == "" || utf8.RuneCountInString(s) > 64 || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return false
	}
	for _, r := range s {
		if r != '-' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// tagSlugFrom 由标签名生成 slug：字母转小写，其他符号和空白变成短横线。
func tagSlugFrom(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimSuffix(b.String(), "-")
	if r := []rune(s); len(r) > 60 {
		s = strings.TrimSuffix(string(r[:60]), "-")
	}
	if s == "" {
		s = "tag"
	}
	return s
}

// MediaLookup 解析封面和图标。
type MediaLookup interface {
	GetMany(ctx context.Context, ids []int64) (map[int64]media.Media, error)
	BaseURL() string
}

// UserLookup 查询作者昵称。
type UserLookup interface {
	Nicknames(ctx context.Context, ids []int64) (map[int64]string, error)
}

// Auditor 记录后台操作日志。
type Auditor interface {
	Record(ctx context.Context, adminID int64, action, targetType string, targetID int64, detail any, ip string) error
}

type Service struct {
	db    *sql.DB
	q     *dbgen.Queries
	media MediaLookup
	users UserLookup
	audit Auditor
	now   func() time.Time
}

func NewService(db *sql.DB, media MediaLookup, users UserLookup, audit Auditor) *Service {
	return &Service{db: db, q: dbgen.New(db), media: media, users: users, audit: audit, now: time.Now}
}

func nullInt64(p *int64) sql.NullInt64 {
	if p == nil || *p == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func ptrInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func ptrTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	v := n.Time
	return &v
}

func normPage(page, size, def, maxSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > maxSize {
		size = def
	}
	return page, size
}
