package public

import (
	"context"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/access"
	"xzji/internal/module/comment"
	"xzji/internal/module/user"
)

// identity 当前请求的用户，未登录时 UserID 为 0。
func identity(ctx context.Context) access.Identity {
	id, ok := authn.FromContext(ctx)
	if !ok {
		return access.Identity{}
	}
	return access.Identity{UserID: id.UserID, Admin: id.Role == user.RoleAdmin}
}

func toLock(l *access.Lock) *pubapi.Lock {
	if l == nil {
		return nil
	}
	return &pubapi.Lock{Reason: pubapi.LockReason(l.Reason), Rank: l.Rank}
}

func (h *Handler) ListMembershipLevels(ctx context.Context, _ pubapi.ListMembershipLevelsRequestObject) (pubapi.ListMembershipLevelsResponseObject, error) {
	levels, err := h.Members.ActiveLevels(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.MembershipLevel, len(levels))
	for i, l := range levels {
		items[i] = pubapi.MembershipLevel{Id: l.ID, Name: l.Name, Rank: l.Rank, DiscountRate: l.DiscountRate, DailyDownloadLimit: l.DailyDownloadLimit}
	}
	return pubapi.ListMembershipLevels200JSONResponse{Items: items}, nil
}

func (h *Handler) GetUnlockState(ctx context.Context, req pubapi.GetUnlockStateRequestObject) (pubapi.GetUnlockStateResponseObject, error) {
	st, err := h.Content.Unlocked(ctx, identity(ctx), req.Id)
	if err != nil {
		return nil, err
	}
	v := st.Viewer
	out := pubapi.UnlockState{
		Viewer: pubapi.ViewerState{
			LoggedIn: v.UserID > 0, Privileged: v.Privileged, MemberRank: v.MemberRank,
			Purchased: v.Purchased, Commented: v.Commented, DailyLimit: v.DailyLimit,
		},
		Price: st.Price, CanBuy: st.CanBuy, HiddenBlocks: make([]pubapi.BlockState, len(st.Blocks)),
	}
	for i, b := range st.Blocks {
		bs := pubapi.BlockState{Seq: b.Seq, Unlocked: b.Decision.Allowed, Lock: toLock(b.Decision.Lock)}
		if b.Decision.Allowed {
			html := b.ContentHTML
			bs.ContentHtml = &html
		}
		out.HiddenBlocks[i] = bs
	}
	if d := st.Download; d != nil {
		out.Download = &pubapi.DownloadState{
			Unlocked: d.Decision.Allowed, MemberFree: d.Decision.MemberFree, UsedToday: d.UsedToday, Lock: toLock(d.Decision.Lock),
		}
	}
	return pubapi.GetUnlockState200JSONResponse(out), nil
}

func (h *Handler) DownloadLink(ctx context.Context, req pubapi.DownloadLinkRequestObject) (pubapi.DownloadLinkResponseObject, error) {
	l, err := h.Content.Download(ctx, identity(ctx), ginCtx(ctx).ClientIP(), req.Id, req.LinkId)
	if err != nil {
		return nil, err
	}
	return pubapi.DownloadLink200JSONResponse{
		Url: l.URL, ExtractCode: l.ExtractCode, UnzipPassword: l.UnzipPassword, Status: pubapi.DownloadLinkStatus(l.Status),
	}, nil
}

func (h *Handler) ReportLink(ctx context.Context, req pubapi.ReportLinkRequestObject) (pubapi.ReportLinkResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	if err := h.Content.ReportLink(ctx, id.UserID, req.Id, req.LinkId, deref(req.Body.Reason)); err != nil {
		return nil, err
	}
	return pubapi.ReportLink204Response{}, nil
}

func toCommentAuthor(a comment.Author) pubapi.CommentAuthor {
	out := pubapi.CommentAuthor{Id: a.ID, Nickname: a.Nickname}
	if a.Badge != "" {
		b := a.Badge
		out.Badge = &b
	}
	return out
}

func toComment(c *comment.Comment) pubapi.Comment {
	out := pubapi.Comment{
		Id: c.ID, Content: c.Content, Status: pubapi.CommentStatus(c.Status), CreatedAt: c.CreatedAt,
		Author: toCommentAuthor(c.Author), Replies: make([]pubapi.Comment, len(c.Replies)),
	}
	if c.ReplyTo != nil {
		r := toCommentAuthor(*c.ReplyTo)
		out.ReplyTo = &r
	}
	for i, r := range c.Replies {
		out.Replies[i] = toComment(r)
	}
	return out
}

func (h *Handler) ListComments(ctx context.Context, req pubapi.ListCommentsRequestObject) (pubapi.ListCommentsResponseObject, error) {
	res, err := h.Comments.ListForPost(ctx, req.Id, identity(ctx).UserID, deref(req.Params.Page), deref(req.Params.PageSize))
	if err != nil {
		return nil, err
	}
	items := make([]pubapi.Comment, len(res.Comments))
	for i, c := range res.Comments {
		items[i] = toComment(c)
	}
	return pubapi.ListComments200JSONResponse{Items: items, Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) CreateComment(ctx context.Context, req pubapi.CreateCommentRequestObject) (pubapi.CreateCommentResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	c := ginCtx(ctx)
	created, err := h.Comments.Create(ctx, comment.CreateInput{
		UserID: id.UserID, Admin: id.Role == user.RoleAdmin, PostID: req.Id, Content: req.Body.Content,
		ReplyToID: deref(req.Body.ReplyToId), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		return nil, err
	}
	return pubapi.CreateComment201JSONResponse(toComment(created)), nil
}
