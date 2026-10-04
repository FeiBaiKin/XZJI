package admin

import (
	"context"
	"time"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/comment"
	"xzji/internal/module/membership"
)

// ---------- 会员等级 ----------

func toAdminLevel(l membership.Level) adminapi.AdminLevel {
	return adminapi.AdminLevel{
		Id: l.ID, Name: l.Name, Description: l.Description, Rank: l.Rank, DiscountRate: l.DiscountRate, DailyDownloadLimit: l.DailyDownloadLimit,
		Perks: l.Perks, Status: adminapi.LevelStatus(l.Status), MemberCount: l.MemberCount,
	}
}

func levelInput(b *adminapi.LevelInput) membership.LevelInput {
	return membership.LevelInput{
		Name: b.Name, Description: deref(b.Description), Rank: b.Rank, DiscountRate: b.DiscountRate, DailyDownloadLimit: b.DailyDownloadLimit,
		Perks: deref(b.Perks), Status: string(deref(b.Status)),
	}
}

func (h *Handler) ListMembershipLevels(ctx context.Context, _ adminapi.ListMembershipLevelsRequestObject) (adminapi.ListMembershipLevelsResponseObject, error) {
	levels, err := h.Members.Levels(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminLevel, len(levels))
	for i, l := range levels {
		items[i] = toAdminLevel(l)
	}
	return adminapi.ListMembershipLevels200JSONResponse{Items: items}, nil
}

func (h *Handler) CreateMembershipLevel(ctx context.Context, req adminapi.CreateMembershipLevelRequestObject) (adminapi.CreateMembershipLevelResponseObject, error) {
	l, err := h.Members.CreateLevel(ctx, levelInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.CreateMembershipLevel201JSONResponse(toAdminLevel(l)), nil
}

func (h *Handler) UpdateMembershipLevel(ctx context.Context, req adminapi.UpdateMembershipLevelRequestObject) (adminapi.UpdateMembershipLevelResponseObject, error) {
	l, err := h.Members.UpdateLevel(ctx, req.Id, levelInput(req.Body))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateMembershipLevel200JSONResponse(toAdminLevel(l)), nil
}

func (h *Handler) DeleteMembershipLevel(ctx context.Context, req adminapi.DeleteMembershipLevelRequestObject) (adminapi.DeleteMembershipLevelResponseObject, error) {
	if err := h.Members.DeleteLevel(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteMembershipLevel204Response{}, nil
}

// ---------- 用户的会员 ----------

func toUserMembership(m *membership.Membership) *adminapi.UserMembership {
	if m == nil {
		return nil
	}
	return &adminapi.UserMembership{
		LevelId: m.Level.ID, Name: m.Level.Name, Rank: m.Level.Rank, ExpiresAt: m.ExpiresAt, Active: m.Active(time.Now()),
	}
}

func (h *Handler) SetUserMembership(ctx context.Context, req adminapi.SetUserMembershipRequestObject) (adminapi.SetUserMembershipResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	if _, err := h.Members.Grant(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.Id, req.Body.LevelId, req.Body.ExpiresAt); err != nil {
		return nil, err
	}
	u, err := h.Users.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	item, err := h.userItem(ctx, u)
	if err != nil {
		return nil, err
	}
	return adminapi.SetUserMembership200JSONResponse(item), nil
}

func (h *Handler) DeleteUserMembership(ctx context.Context, req adminapi.DeleteUserMembershipRequestObject) (adminapi.DeleteUserMembershipResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	if err := h.Members.Revoke(ctx, id.UserID, ginCtx(ctx).ClientIP(), req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteUserMembership204Response{}, nil
}

// ---------- 评论 ----------

func toAdminComment(c *comment.Comment) adminapi.AdminComment {
	out := adminapi.AdminComment{
		Id: c.ID, PostId: c.PostID, PostTitle: c.PostTitle, ParentId: c.ParentID, Content: c.Content,
		Status: adminapi.CommentStatus(c.Status), Ip: c.IP, CreatedAt: c.CreatedAt,
		Author: adminapi.Author{Id: c.Author.ID, Nickname: c.Author.Nickname},
	}
	if c.ReplyTo != nil {
		out.ReplyTo = &adminapi.Author{Id: c.ReplyTo.ID, Nickname: c.ReplyTo.Nickname}
	}
	return out
}

func (h *Handler) ListComments(ctx context.Context, req adminapi.ListCommentsRequestObject) (adminapi.ListCommentsResponseObject, error) {
	p := req.Params
	res, err := h.Comments.AdminList(ctx, comment.AdminFilter{
		Status: comment.Status(deref(p.Status)), PostID: deref(p.PostId), Keyword: deref(p.Keyword),
		Page: deref(p.Page), PageSize: deref(p.PageSize),
	})
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.AdminComment, len(res.Comments))
	for i, c := range res.Comments {
		items[i] = toAdminComment(c)
	}
	return adminapi.ListComments200JSONResponse{Items: items, Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) UpdateComment(ctx context.Context, req adminapi.UpdateCommentRequestObject) (adminapi.UpdateCommentResponseObject, error) {
	c, err := h.Comments.SetStatus(ctx, req.Id, comment.Status(req.Body.Status))
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateComment200JSONResponse(toAdminComment(c)), nil
}

func (h *Handler) DeleteComment(ctx context.Context, req adminapi.DeleteCommentRequestObject) (adminapi.DeleteCommentResponseObject, error) {
	if err := h.Comments.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return adminapi.DeleteComment204Response{}, nil
}

func (h *Handler) GetCommentSettings(ctx context.Context, _ adminapi.GetCommentSettingsRequestObject) (adminapi.GetCommentSettingsResponseObject, error) {
	m, err := h.Comments.Moderation(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.GetCommentSettings200JSONResponse{Moderation: adminapi.CommentSettingsModeration(m)}, nil
}

func (h *Handler) UpdateCommentSettings(ctx context.Context, req adminapi.UpdateCommentSettingsRequestObject) (adminapi.UpdateCommentSettingsResponseObject, error) {
	if err := h.Comments.SetModeration(ctx, comment.Moderation(req.Body.Moderation)); err != nil {
		return nil, err
	}
	return adminapi.UpdateCommentSettings200JSONResponse{Moderation: req.Body.Moderation}, nil
}

// ---------- 失效反馈 ----------

func (h *Handler) ListLinkReports(ctx context.Context, req adminapi.ListLinkReportsRequestObject) (adminapi.ListLinkReportsResponseObject, error) {
	p := req.Params
	res, err := h.Content.LinkReports(ctx, string(deref(p.Status)), deref(p.Page), deref(p.PageSize))
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.LinkReport, len(res.Reports))
	for i, r := range res.Reports {
		items[i] = adminapi.LinkReport{
			Id: r.ID, PostId: r.PostID, PostTitle: r.PostTitle, LinkId: r.LinkID, LinkName: r.LinkName, LinkUrl: r.LinkURL,
			LinkStatus: r.LinkStatus, User: adminapi.Author{Id: r.UserID, Nickname: r.Nickname}, Reason: r.Reason,
			Status: adminapi.ReportStatus(r.Status), HandledAt: r.HandledAt, CreatedAt: r.CreatedAt,
		}
	}
	return adminapi.ListLinkReports200JSONResponse{Items: items, Total: res.Total, Page: res.Page, PageSize: res.PageSize}, nil
}

func (h *Handler) UpdateLinkReport(ctx context.Context, req adminapi.UpdateLinkReportRequestObject) (adminapi.UpdateLinkReportResponseObject, error) {
	if err := h.Content.SetLinkReportStatus(ctx, req.Id, string(req.Body.Status)); err != nil {
		return nil, err
	}
	return adminapi.UpdateLinkReport204Response{}, nil
}
