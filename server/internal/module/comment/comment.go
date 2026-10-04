// Package comment 管理评论：发表、两级回复、审核，以及“评论可见”的判定依据。
package comment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"xzji/internal/dbgen"
	"xzji/internal/module/setting"
	platformdb "xzji/internal/platform/db"
	"xzji/internal/platform/errs"
	"xzji/internal/platform/ratelimit"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusSpam     Status = "spam"
)

// Moderation 审核方式：all 全部先审核；trusted 有过审核通过评论的用户免审；none 不审核。
type Moderation string

const (
	ModerationAll     Moderation = "all"
	ModerationTrusted Moderation = "trusted"
	ModerationNone    Moderation = "none"
)

const (
	KeyModeration = "comment.moderation"
	minLen        = 2
	maxLen        = 1000
	// 同一用户 15 秒内只能发一条，每天最多 50 条
	interval   = 15 * time.Second
	dailyLimit = 50
)

var (
	ErrNotFound        = errs.NotFound("COMMENT_NOT_FOUND", "评论不存在")
	ErrTooShort        = errs.Invalid("COMMENT_TOO_SHORT", "评论至少 2 个字")
	ErrTooLong         = errs.Invalid("COMMENT_TOO_LONG", "评论不能超过 1000 个字")
	ErrInvalidReply    = errs.Invalid("INVALID_REPLY", "要回复的评论不存在或还没通过审核")
	ErrInvalidStatus   = errs.Invalid("INVALID_STATUS", "评论状态不正确")
	ErrInvalidModerate = errs.Invalid("INVALID_MODERATION", "审核方式不正确")
	ErrTooFrequent     = errs.TooMany("评论太频繁了，请稍后再发")
	ErrDailyLimit      = errs.TooMany("今天的评论次数已经用完了，明天再来吧")
)

type Author struct {
	ID       int64
	Nickname string
	// Badge 有效会员的等级名称，非会员为空
	Badge string
}

type Comment struct {
	ID        int64
	PostID    int64
	ParentID  *int64
	Content   string
	Status    Status
	IP        string
	CreatedAt time.Time
	Author    Author
	ReplyTo   *Author
	Replies   []*Comment
	// PostTitle 只在后台列表中填充
	PostTitle string
}

// PostLookup 内容模块提供的能力。
type PostLookup interface {
	// CommentablePost 内容存在且已发布时返回 nil
	CommentablePost(ctx context.Context, postID int64) error
	PostTitles(ctx context.Context, ids []int64) (map[int64]string, error)
	SetCommentCount(ctx context.Context, postID int64, n int) error
}

type UserLookup interface {
	Nicknames(ctx context.Context, ids []int64) (map[int64]string, error)
}

// BadgeLookup 查询有效会员的等级名称。
type BadgeLookup interface {
	Badges(ctx context.Context, ids []int64) (map[int64]string, error)
}

// Purger 清空前台缓存，由 frontsync.Service 实现：评论审核通过后评论数会变，详情页缓存要失效。
type Purger interface {
	Purge()
}

type Service struct {
	q        *dbgen.Queries
	posts    PostLookup
	users    UserLookup
	badges   BadgeLookup
	settings *setting.Service
	limiter  ratelimit.Limiter
	purger   Purger
}

func NewService(q *dbgen.Queries, posts PostLookup, users UserLookup, badges BadgeLookup, settings *setting.Service, limiter ratelimit.Limiter) *Service {
	return &Service{q: q, posts: posts, users: users, badges: badges, settings: settings, limiter: limiter}
}

// SetPurger comment 与 frontsync 互相依赖，装配后再注入。
func (s *Service) SetPurger(p Purger) { s.purger = p }

func (s *Service) Moderation(ctx context.Context) (Moderation, error) {
	var m Moderation
	err := s.settings.Get(ctx, KeyModeration, &m)
	if errors.Is(err, setting.ErrNotFound) {
		return ModerationAll, nil
	}
	return m, err
}

func (s *Service) SetModeration(ctx context.Context, m Moderation) error {
	if m != ModerationAll && m != ModerationTrusted && m != ModerationNone {
		return ErrInvalidModerate
	}
	return s.settings.Set(ctx, KeyModeration, m)
}

// HasApproved 实现 access.CommentChecker。
func (s *Service) HasApproved(ctx context.Context, userID, postID int64) (bool, error) {
	ok, err := s.q.HasApprovedComment(ctx, dbgen.HasApprovedCommentParams{UserID: userID, PostID: postID})
	if err != nil {
		return false, fmt.Errorf("查询评论记录：%w", err)
	}
	return ok, nil
}

type CreateInput struct {
	UserID    int64
	Admin     bool
	PostID    int64
	Content   string
	ReplyToID int64
	IP        string
	UserAgent string
}

// Create 发表评论或回复。回复楼中楼时挂在同一条一级评论下，并记录被回复的人。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Comment, error) {
	content := strings.TrimSpace(in.Content)
	if n := utf8.RuneCountInString(content); n < minLen {
		return nil, ErrTooShort
	} else if n > maxLen {
		return nil, ErrTooLong
	}
	if err := s.posts.CommentablePost(ctx, in.PostID); err != nil {
		return nil, err
	}
	intervalKey := fmt.Sprintf("comment:interval:%d", in.UserID)
	dailyKey := fmt.Sprintf("comment:daily:%d", in.UserID)
	if n, _ := s.limiter.Count(intervalKey); n > 0 {
		return nil, ErrTooFrequent
	}
	if n, _ := s.limiter.Count(dailyKey); n >= dailyLimit {
		return nil, ErrDailyLimit
	}

	var parent, replyTo sql.NullInt64
	if in.ReplyToID > 0 {
		target, err := s.q.GetComment(ctx, in.ReplyToID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (target.PostID != in.PostID || Status(target.Status) != StatusApproved)) {
			return nil, ErrInvalidReply
		}
		if err != nil {
			return nil, fmt.Errorf("查询被回复的评论：%w", err)
		}
		parent = sql.NullInt64{Int64: target.ID, Valid: true}
		if target.ParentID.Valid {
			parent = target.ParentID
			replyTo = sql.NullInt64{Int64: target.UserID, Valid: true}
		}
	}

	status, err := s.initialStatus(ctx, in.UserID, in.Admin)
	if err != nil {
		return nil, err
	}
	id, err := s.q.CreateComment(ctx, dbgen.CreateCommentParams{
		PostID: in.PostID, UserID: in.UserID, ParentID: parent, ReplyToUserID: replyTo,
		Content: content, Status: string(status), Ip: in.IP, UserAgent: truncate(in.UserAgent, 255),
	})
	if err != nil {
		return nil, fmt.Errorf("保存评论：%w", err)
	}
	s.limiter.Incr(intervalKey, interval)
	s.limiter.Incr(dailyKey, 24*time.Hour)
	if status == StatusApproved {
		if err := s.refreshCount(ctx, in.PostID); err != nil {
			return nil, err
		}
		// 评论数变了，前台缓存的详情页要立刻失效
		if s.purger != nil {
			s.purger.Purge()
		}
	}
	return s.get(ctx, id)
}

func (s *Service) initialStatus(ctx context.Context, userID int64, admin bool) (Status, error) {
	if admin {
		return StatusApproved, nil
	}
	m, err := s.Moderation(ctx)
	if err != nil {
		return "", err
	}
	switch m {
	case ModerationNone:
		return StatusApproved, nil
	case ModerationTrusted:
		ok, err := s.q.UserHasApprovedComment(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("查询评论记录：%w", err)
		}
		if ok {
			return StatusApproved, nil
		}
	}
	return StatusPending, nil
}

func (s *Service) refreshCount(ctx context.Context, postID int64) error {
	n, err := s.q.CountApprovedComments(ctx, postID)
	if err != nil {
		return fmt.Errorf("统计评论数：%w", err)
	}
	return s.posts.SetCommentCount(ctx, postID, int(n))
}

func (s *Service) get(ctx context.Context, id int64) (*Comment, error) {
	row, err := s.q.GetComment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询评论 %d：%w", id, err)
	}
	list, err := s.hydrate(ctx, []dbgen.Comment{row})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

// hydrate 补齐作者昵称、会员标识和被回复人的昵称。
func (s *Service) hydrate(ctx context.Context, rows []dbgen.Comment) ([]*Comment, error) {
	ids := make([]int64, 0, len(rows)*2)
	for _, r := range rows {
		ids = append(ids, r.UserID)
		if r.ReplyToUserID.Valid {
			ids = append(ids, r.ReplyToUserID.Int64)
		}
	}
	names, err := s.users.Nicknames(ctx, ids)
	if err != nil {
		return nil, err
	}
	badges, err := s.badges.Badges(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*Comment, len(rows))
	for i, r := range rows {
		c := &Comment{
			ID: r.ID, PostID: r.PostID, Content: r.Content, Status: Status(r.Status), IP: r.Ip, CreatedAt: r.CreatedAt,
			Author:  Author{ID: r.UserID, Nickname: names[r.UserID], Badge: badges[r.UserID]},
			Replies: []*Comment{},
		}
		if r.ParentID.Valid {
			p := r.ParentID.Int64
			c.ParentID = &p
		}
		if r.ReplyToUserID.Valid {
			u := r.ReplyToUserID.Int64
			c.ReplyTo = &Author{ID: u, Nickname: names[u], Badge: badges[u]}
		}
		out[i] = c
	}
	return out, nil
}

type Page struct {
	Comments []*Comment
	// Total 一级评论数
	Total    int64
	Page     int
	PageSize int
}

// ListForPost 前台评论列表：一级评论倒序分页，回复按时间顺序挂在下面。
// 登录用户还能看到自己待审核的评论。
func (s *Service) ListForPost(ctx context.Context, postID, viewerID int64, page, pageSize int) (Page, error) {
	if err := s.posts.CommentablePost(ctx, postID); err != nil {
		return Page{}, err
	}
	page = max(page, 1)
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}
	roots, err := s.q.ListRootComments(ctx, dbgen.ListRootCommentsParams{
		PostID: postID, ViewerID: viewerID, Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return Page{}, fmt.Errorf("查询评论：%w", err)
	}
	total, err := s.q.CountRootComments(ctx, dbgen.CountRootCommentsParams{PostID: postID, ViewerID: viewerID})
	if err != nil {
		return Page{}, fmt.Errorf("统计评论：%w", err)
	}
	rows := roots
	if len(roots) > 0 {
		ids := make([]sql.NullInt64, len(roots))
		for i, r := range roots {
			ids[i] = sql.NullInt64{Int64: r.ID, Valid: true}
		}
		replies, err := s.q.ListCommentReplies(ctx, dbgen.ListCommentRepliesParams{ParentIds: ids, ViewerID: viewerID})
		if err != nil {
			return Page{}, fmt.Errorf("查询回复：%w", err)
		}
		rows = append(append([]dbgen.Comment{}, roots...), replies...)
	}
	all, err := s.hydrate(ctx, rows)
	if err != nil {
		return Page{}, err
	}
	byID := make(map[int64]*Comment, len(roots))
	list := all[:len(roots)]
	for _, c := range list {
		byID[c.ID] = c
	}
	for _, c := range all[len(roots):] {
		if p := byID[*c.ParentID]; p != nil {
			p.Replies = append(p.Replies, c)
		}
	}
	return Page{Comments: list, Total: total, Page: page, PageSize: pageSize}, nil
}

type AdminFilter struct {
	Status   Status
	PostID   int64
	Keyword  string
	Page     int
	PageSize int
}

func (s *Service) AdminList(ctx context.Context, f AdminFilter) (Page, error) {
	f.Page = max(f.Page, 1)
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	var status, kw sql.NullString
	var post sql.NullInt64
	if f.Status != "" {
		status = sql.NullString{String: string(f.Status), Valid: true}
	}
	if k := strings.TrimSpace(f.Keyword); k != "" {
		kw = sql.NullString{String: platformdb.LikeContains(k), Valid: true}
	}
	if f.PostID > 0 {
		post = sql.NullInt64{Int64: f.PostID, Valid: true}
	}
	rows, err := s.q.ListCommentsAdmin(ctx, dbgen.ListCommentsAdminParams{
		Status: status, PostID: post, Keyword: kw, Limit: int32(f.PageSize), Offset: int32((f.Page - 1) * f.PageSize),
	})
	if err != nil {
		return Page{}, fmt.Errorf("查询评论：%w", err)
	}
	total, err := s.q.CountCommentsAdmin(ctx, dbgen.CountCommentsAdminParams{Status: status, PostID: post, Keyword: kw})
	if err != nil {
		return Page{}, fmt.Errorf("统计评论：%w", err)
	}
	list, err := s.hydrate(ctx, rows)
	if err != nil {
		return Page{}, err
	}
	postIDs := make([]int64, len(list))
	for i, c := range list {
		postIDs[i] = c.PostID
	}
	titles, err := s.posts.PostTitles(ctx, postIDs)
	if err != nil {
		return Page{}, err
	}
	for _, c := range list {
		c.PostTitle = titles[c.PostID]
	}
	return Page{Comments: list, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// SetStatus 审核评论，并重新统计这篇内容的评论数。
func (s *Service) SetStatus(ctx context.Context, id int64, status Status) (*Comment, error) {
	if status != StatusPending && status != StatusApproved && status != StatusSpam {
		return nil, ErrInvalidStatus
	}
	c, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.q.UpdateCommentStatus(ctx, dbgen.UpdateCommentStatusParams{Status: string(status), ID: id}); err != nil {
		return nil, fmt.Errorf("修改评论状态：%w", err)
	}
	if err := s.refreshCount(ctx, c.PostID); err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

// Delete 删除评论；删除一级评论时连同它下面的回复一起删除。
func (s *Service) Delete(ctx context.Context, id int64) error {
	c, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	if c.ParentID == nil {
		if err := s.q.DeleteCommentReplies(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
			return fmt.Errorf("删除回复：%w", err)
		}
	}
	if err := s.q.DeleteComment(ctx, id); err != nil {
		return fmt.Errorf("删除评论：%w", err)
	}
	return s.refreshCount(ctx, c.PostID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// 按字节截断时退回到完整字符的边界
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
