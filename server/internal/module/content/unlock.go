package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"xzji/internal/dbgen"
	"xzji/internal/module/access"
	"xzji/internal/platform/errs"
)

// 每个用户（未登录按 IP）每小时最多获取 60 次下载地址，防止批量抓取
const downloadsPerHour = 60

// BlockState 一个隐藏块对当前访问者的解锁情况，未解锁时 ContentHTML 为空。
type BlockState struct {
	Seq         int
	Decision    access.Decision
	ContentHTML string
}

type DownloadState struct {
	Decision access.Decision
	// UsedToday 今天已用的会员免费下载次数，只在有每日次数限制时统计
	UsedToday int
}

// UnlockState 当前访问者在一篇内容上能看到哪些受保护的部分。每次请求实时判定，不缓存。
type UnlockState struct {
	Viewer access.Viewer
	// Price 单独购买需要支付的金额，CanBuy 为 false 时无意义
	Price    int64
	CanBuy   bool
	Blocks   []BlockState
	Download *DownloadState
}

func rule(row dbgen.Post) access.Rule {
	return access.Rule{Price: row.Price, MemberFreeRank: int(row.MemberFreeRank), MemberDiscount: row.MemberDiscount}
}

// readable 前台可以访问这篇内容：已发布，或者访问者是管理员（预览草稿）。
func (s *Service) readable(ctx context.Context, id access.Identity, postID int64) (dbgen.Post, error) {
	row, err := s.getRow(ctx, postID)
	if err != nil {
		return row, err
	}
	if !s.visible(row) && !(id.Admin && Status(row.Status) != StatusTrash) {
		return row, ErrPostNotFound
	}
	return row, nil
}

func (s *Service) dayStart() time.Time {
	now := s.now().In(s.loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc).UTC()
}

func (s *Service) Unlocked(ctx context.Context, id access.Identity, postID int64) (*UnlockState, error) {
	row, err := s.readable(ctx, id, postID)
	if err != nil {
		return nil, err
	}
	v, err := s.viewers.Viewer(ctx, id, row.ID, row.AuthorID)
	if err != nil {
		return nil, err
	}
	r := rule(row)
	st := &UnlockState{Viewer: v, Blocks: []BlockState{}}
	st.Price, st.CanBuy = access.Price(r, v)

	blocks, err := s.q.ListHiddenBlocks(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("查询隐藏内容：%w", err)
	}
	for _, b := range blocks {
		cond, err := access.ParseCond(b.Cond)
		if err != nil {
			return nil, fmt.Errorf("隐藏块 %d 的解锁条件 %q 不正确：%w", b.Seq, b.Cond, err)
		}
		bs := BlockState{Seq: int(b.Seq), Decision: access.Check(cond, r, v)}
		if bs.Decision.Allowed {
			bs.ContentHTML = outputHTML(b.ContentHtml, s.media.BaseURL())
		}
		st.Blocks = append(st.Blocks, bs)
	}

	if Type(row.Type) == TypeResource {
		res, err := s.q.GetResource(ctx, row.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("查询资源信息：%w", err)
		}
		cond, err := access.ParseCond(res.DownloadCond)
		if err != nil {
			cond = access.Cond{Kind: access.KindPublic}
		}
		ds := &DownloadState{Decision: access.Check(cond, r, v)}
		if ds.Decision.MemberFree && v.DailyLimit > 0 {
			n, err := s.q.CountMemberFreeDownloadsSince(ctx, dbgen.CountMemberFreeDownloadsSinceParams{UserID: v.UserID, CreatedAt: s.dayStart()})
			if err != nil {
				return nil, fmt.Errorf("统计今日下载次数：%w", err)
			}
			ds.UsedToday = int(n)
		}
		st.Download = ds
	}
	return st, nil
}

func lockedError(l *access.Lock) error {
	msg := map[access.Kind]string{
		access.KindComment: "评论审核通过后才能下载",
		access.KindPaid:    "购买后才能下载",
		access.KindMember:  "开通会员后才能下载",
	}[l.Reason]
	if msg == "" {
		return ErrLoginRequired
	}
	return errs.New(errs.KindForbidden, "DOWNLOAD_LOCKED", msg)
}

// Download 获取下载地址：判定下载区的解锁条件，靠会员身份免费下载的检查每日次数，并记录下载。
// 同一篇内容当天重复下载不重复计数。
func (s *Service) Download(ctx context.Context, id access.Identity, ip string, postID, linkID int64) (Link, error) {
	row, err := s.readable(ctx, id, postID)
	if err != nil {
		return Link{}, err
	}
	if Type(row.Type) != TypeResource {
		return Link{}, ErrLinkNotFound
	}
	l, err := s.q.GetResourceLink(ctx, dbgen.GetResourceLinkParams{ID: linkID, PostID: postID})
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrLinkNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("查询下载地址：%w", err)
	}
	res, err := s.q.GetResource(ctx, postID)
	if err != nil {
		return Link{}, fmt.Errorf("查询资源信息：%w", err)
	}
	cond, err := access.ParseCond(res.DownloadCond)
	if err != nil {
		return Link{}, fmt.Errorf("下载区的解锁条件 %q 不正确：%w", res.DownloadCond, err)
	}
	v, err := s.viewers.Viewer(ctx, id, row.ID, row.AuthorID)
	if err != nil {
		return Link{}, err
	}
	d := access.Check(cond, rule(row), v)
	if !d.Allowed {
		if v.UserID == 0 {
			return Link{}, ErrLoginRequired
		}
		return Link{}, lockedError(d.Lock)
	}

	key := "download:ip:" + ip
	if v.UserID > 0 {
		key = fmt.Sprintf("download:user:%d", v.UserID)
	}
	if n, _ := s.limiter.Incr(key, time.Hour); n > downloadsPerHour {
		return Link{}, errs.TooMany("获取下载地址太频繁了，请稍后再试")
	}

	if d.MemberFree && v.DailyLimit > 0 {
		since := s.dayStart()
		again, err := s.q.HasMemberFreeDownloadSince(ctx, dbgen.HasMemberFreeDownloadSinceParams{UserID: v.UserID, PostID: postID, CreatedAt: since})
		if err != nil {
			return Link{}, fmt.Errorf("查询今日下载记录：%w", err)
		}
		if !again {
			n, err := s.q.CountMemberFreeDownloadsSince(ctx, dbgen.CountMemberFreeDownloadsSinceParams{UserID: v.UserID, CreatedAt: since})
			if err != nil {
				return Link{}, fmt.Errorf("统计今日下载次数：%w", err)
			}
			if int(n) >= v.DailyLimit {
				return Link{}, ErrDailyLimit
			}
		}
	}
	if err := s.q.CreateDownloadLog(ctx, dbgen.CreateDownloadLogParams{
		UserID: v.UserID, PostID: postID, LinkID: linkID, IsMemberFree: d.MemberFree, Ip: ip,
	}); err != nil {
		return Link{}, fmt.Errorf("记录下载：%w", err)
	}
	return Link{ID: l.ID, Name: l.Name, URL: l.Url, ExtractCode: l.ExtractCode, UnzipPassword: l.UnzipPassword, Status: l.Status}, nil
}

// ReportLink 用户反馈下载地址失效。同一用户对同一地址只保留一条待处理的反馈。
func (s *Service) ReportLink(ctx context.Context, userID int64, postID, linkID int64, reason string) error {
	if _, err := s.readable(ctx, access.Identity{UserID: userID}, postID); err != nil {
		return err
	}
	if _, err := s.q.GetResourceLink(ctx, dbgen.GetResourceLinkParams{ID: linkID, PostID: postID}); errors.Is(err, sql.ErrNoRows) {
		return ErrLinkNotFound
	} else if err != nil {
		return fmt.Errorf("查询下载地址：%w", err)
	}
	reason, err := checkLen("反馈说明", reason, 200)
	if err != nil {
		return err
	}
	dup, err := s.q.HasPendingLinkReport(ctx, dbgen.HasPendingLinkReportParams{LinkID: linkID, UserID: userID})
	if err != nil {
		return fmt.Errorf("查询失效反馈：%w", err)
	}
	if dup {
		return ErrAlreadyReported
	}
	if n, _ := s.limiter.Incr(fmt.Sprintf("link-report:%d", userID), time.Hour); n > 10 {
		return errs.TooMany("反馈太频繁了，请稍后再试")
	}
	if err := s.q.CreateLinkReport(ctx, dbgen.CreateLinkReportParams{LinkID: linkID, PostID: postID, UserID: userID, Reason: reason}); err != nil {
		return fmt.Errorf("保存失效反馈：%w", err)
	}
	return nil
}

type LinkReport struct {
	ID         int64
	PostID     int64
	PostTitle  string
	LinkID     int64
	LinkName   string
	LinkURL    string
	LinkStatus string
	UserID     int64
	Nickname   string
	Reason     string
	Status     string
	HandledAt  *time.Time
	CreatedAt  time.Time
}

type ReportPage struct {
	Reports  []LinkReport
	Total    int64
	Page     int
	PageSize int
}

func (s *Service) LinkReports(ctx context.Context, status string, page, pageSize int) (ReportPage, error) {
	page, pageSize = normPage(page, pageSize, 20, 100)
	var st sql.NullString
	if status != "" {
		st = sql.NullString{String: status, Valid: true}
	}
	rows, err := s.q.ListLinkReports(ctx, dbgen.ListLinkReportsParams{Status: st, Limit: int32(pageSize), Offset: int32((page - 1) * pageSize)})
	if err != nil {
		return ReportPage{}, fmt.Errorf("查询失效反馈：%w", err)
	}
	total, err := s.q.CountLinkReports(ctx, dbgen.CountLinkReportsParams{Status: st})
	if err != nil {
		return ReportPage{}, fmt.Errorf("统计失效反馈：%w", err)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.UserID
	}
	names, err := s.users.Nicknames(ctx, ids)
	if err != nil {
		return ReportPage{}, err
	}
	out := ReportPage{Reports: make([]LinkReport, len(rows)), Total: total, Page: page, PageSize: pageSize}
	for i, r := range rows {
		out.Reports[i] = LinkReport{
			ID: r.ID, PostID: r.PostID, PostTitle: r.PostTitle, LinkID: r.LinkID, LinkName: r.LinkName, LinkURL: r.LinkUrl,
			LinkStatus: r.LinkStatus, UserID: r.UserID, Nickname: names[r.UserID], Reason: r.Reason, Status: r.Status,
			HandledAt: ptrTime(r.HandledAt), CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

// SetLinkReportStatus 处理失效反馈：resolved 已补链接，ignored 忽略，pending 重新打开。
func (s *Service) SetLinkReportStatus(ctx context.Context, id int64, status string) error {
	if status != "pending" && status != "resolved" && status != "ignored" {
		return errs.Invalid("INVALID_STATUS", "处理状态不正确")
	}
	if _, err := s.q.GetLinkReport(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return errs.NotFound("REPORT_NOT_FOUND", "反馈不存在")
	} else if err != nil {
		return fmt.Errorf("查询失效反馈：%w", err)
	}
	var handled sql.NullTime
	if status != "pending" {
		handled = sql.NullTime{Time: s.now().UTC(), Valid: true}
	}
	if err := s.q.UpdateLinkReportStatus(ctx, dbgen.UpdateLinkReportStatusParams{Status: status, HandledAt: handled, ID: id}); err != nil {
		return fmt.Errorf("修改失效反馈：%w", err)
	}
	return nil
}

// CommentablePost 实现 comment.PostLookup：只能评论已发布的内容。
func (s *Service) CommentablePost(ctx context.Context, postID int64) error {
	row, err := s.getRow(ctx, postID)
	if err != nil {
		return err
	}
	if !s.visible(row) {
		return ErrPostNotFound
	}
	return nil
}

func (s *Service) PostTitles(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ListPostTitles(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("查询内容标题：%w", err)
	}
	for _, r := range rows {
		out[r.ID] = r.Title
	}
	return out, nil
}

func (s *Service) SetCommentCount(ctx context.Context, postID int64, n int) error {
	if err := s.q.SetPostCommentCount(ctx, dbgen.SetPostCommentCountParams{CommentCount: int32(n), ID: postID}); err != nil {
		return fmt.Errorf("更新评论数：%w", err)
	}
	return nil
}

// PurchaseInfo 下单时需要的内容信息。
type PurchaseInfo struct {
	Title    string
	AuthorID int64
	Rule     access.Rule
}

// PurchaseInfo 只有前台可见的内容才能购买。
func (s *Service) PurchaseInfo(ctx context.Context, postID int64) (PurchaseInfo, error) {
	row, err := s.getRow(ctx, postID)
	if err != nil {
		return PurchaseInfo{}, err
	}
	if !s.visible(row) {
		return PurchaseInfo{}, ErrPostNotFound
	}
	return PurchaseInfo{Title: row.Title, AuthorID: row.AuthorID, Rule: rule(row)}, nil
}

// Cards 按 ID 批量查询内容卡片（包括已下线的），用于已购列表。
func (s *Service) Cards(ctx context.Context, ids []int64) (map[int64]*Post, error) {
	out := make(map[int64]*Post, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	query, args, err := sq.Select(listColumns...).From("posts p").LeftJoin("resources r ON r.post_id = p.id").
		Where(sq.Eq{"p.id": ids}).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询内容：%w", err)
	}
	defer rows.Close()
	var list []listRow
	for rows.Next() {
		var r listRow
		if err := rows.Scan(r.dest()...); err != nil {
			return nil, fmt.Errorf("读取内容：%w", err)
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取内容：%w", err)
	}
	posts, err := s.hydrate(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, p := range posts {
		out[p.ID] = p
	}
	return out, nil
}

// UsedToday 今天已用的会员免费下载次数（同一篇内容当天只算一次）。
func (s *Service) UsedToday(ctx context.Context, userID int64) (int, error) {
	n, err := s.q.CountMemberFreeDownloadsSince(ctx, dbgen.CountMemberFreeDownloadsSinceParams{UserID: userID, CreatedAt: s.dayStart()})
	if err != nil {
		return 0, fmt.Errorf("统计今日下载次数：%w", err)
	}
	return int(n), nil
}
