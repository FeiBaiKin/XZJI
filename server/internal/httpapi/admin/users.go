package admin

import (
	"context"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/user"
)

func (h *Handler) ListUsers(ctx context.Context, req adminapi.ListUsersRequestObject) (adminapi.ListUsersResponseObject, error) {
	p := req.Params
	f := user.ListFilter{Page: 1, PageSize: 20}
	if p.Keyword != nil {
		f.Keyword = *p.Keyword
	}
	if p.Role != nil {
		f.Role = user.Role(*p.Role)
	}
	if p.Status != nil {
		f.Status = user.Status(*p.Status)
	}
	if p.Page != nil {
		f.Page = *p.Page
	}
	if p.PageSize != nil {
		f.PageSize = *p.PageSize
	}
	users, total, err := h.Users.List(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]adminapi.UserItem, len(users))
	for i, u := range users {
		items[i] = toUserItem(u)
	}
	return adminapi.ListUsers200JSONResponse{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

func (h *Handler) UpdateUser(ctx context.Context, req adminapi.UpdateUserRequestObject) (adminapi.UpdateUserResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	var ch user.AdminChange
	if req.Body.Role != nil {
		r := user.Role(*req.Body.Role)
		ch.Role = &r
	}
	if req.Body.Status != nil {
		s := user.Status(*req.Body.Status)
		ch.Status = &s
	}
	u, err := h.Users.AdminUpdate(ctx, user.Actor{ID: id.UserID, IP: ginCtx(ctx).ClientIP()}, req.Id, ch)
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateUser200JSONResponse(toUserItem(u)), nil
}

func toUserItem(u user.User) adminapi.UserItem {
	return adminapi.UserItem{
		Id:            u.ID,
		Email:         u.Email,
		Nickname:      u.Nickname,
		Role:          adminapi.Role(u.Role),
		Status:        adminapi.UserStatus(u.Status),
		EmailVerified: u.EmailVerified,
		InviterId:     u.InviterID,
		LastLoginAt:   u.LastLoginAt,
		CreatedAt:     u.CreatedAt,
	}
}
