package admin

import (
	"context"
	"errors"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/auth"
	"xzji/internal/module/user"
)

func (h *Handler) GetCsrfToken(ctx context.Context, _ adminapi.GetCsrfTokenRequestObject) (adminapi.GetCsrfTokenResponseObject, error) {
	return adminapi.GetCsrfToken200JSONResponse{Token: h.CSRF.Issue(ginCtx(ctx))}, nil
}

// GetSession 未登录时返回空状态，不报错。
func (h *Handler) GetSession(ctx context.Context, _ adminapi.GetSessionRequestObject) (adminapi.GetSessionResponseObject, error) {
	id, ok := authn.FromContext(ctx)
	if !ok {
		return adminapi.GetSession200JSONResponse{}, nil
	}
	u, err := h.Users.Get(ctx, id.UserID)
	if errors.Is(err, user.ErrNotFound) {
		return adminapi.GetSession200JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}
	p := toProfile(u)
	return adminapi.GetSession200JSONResponse{Admin: &p}, nil
}

func (h *Handler) Login(ctx context.Context, req adminapi.LoginRequestObject) (adminapi.LoginResponseObject, error) {
	c := ginCtx(ctx)
	res, err := h.Auth.AdminLogin(c, req.Body.Email, req.Body.Password, auth.Meta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		return nil, err
	}
	h.Realm.Cookies.SetSession(c, res.Token, res.Session.TTL)
	return adminapi.Login200JSONResponse(toProfile(res.User)), nil
}

func (h *Handler) Logout(ctx context.Context, _ adminapi.LogoutRequestObject) (adminapi.LogoutResponseObject, error) {
	c := ginCtx(ctx)
	token, _ := h.Realm.Token(c)
	if err := h.Auth.Logout(c, token); err != nil {
		return nil, err
	}
	h.Realm.Cookies.ClearSession(c)
	return adminapi.Logout204Response{}, nil
}

func (h *Handler) GetMe(ctx context.Context, _ adminapi.GetMeRequestObject) (adminapi.GetMeResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	u, err := h.Users.Get(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	return adminapi.GetMe200JSONResponse(toProfile(u)), nil
}

func (h *Handler) ChangePassword(ctx context.Context, req adminapi.ChangePasswordRequestObject) (adminapi.ChangePasswordResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	if err := h.Users.ChangePassword(ctx, id.UserID, req.Body.OldPassword, req.Body.NewPassword, id.SessionID); err != nil {
		return nil, err
	}
	return adminapi.ChangePassword204Response{}, nil
}

func toProfile(u user.User) adminapi.AdminProfile {
	return adminapi.AdminProfile{Id: u.ID, Email: u.Email, Nickname: u.Nickname}
}
