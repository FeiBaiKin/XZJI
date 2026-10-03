package public

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/auth"
	"xzji/internal/module/session"
	"xzji/internal/module/user"
	"xzji/internal/module/verifycode"
)

func (h *Handler) GetCsrfToken(ctx context.Context, _ pubapi.GetCsrfTokenRequestObject) (pubapi.GetCsrfTokenResponseObject, error) {
	return pubapi.GetCsrfToken200JSONResponse{Token: h.CSRF.Issue(ginCtx(ctx))}, nil
}

// GetSession 未登录或账号已不存在时返回空状态，不报错。
func (h *Handler) GetSession(ctx context.Context, _ pubapi.GetSessionRequestObject) (pubapi.GetSessionResponseObject, error) {
	id, ok := authn.FromContext(ctx)
	if !ok {
		return pubapi.GetSession200JSONResponse{}, nil
	}
	u, err := h.Users.Get(ctx, id.UserID)
	if errors.Is(err, user.ErrNotFound) {
		return pubapi.GetSession200JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := toUser(u)
	return pubapi.GetSession200JSONResponse{User: &out}, nil
}

func (h *Handler) SendEmailCode(ctx context.Context, req pubapi.SendEmailCodeRequestObject) (pubapi.SendEmailCodeResponseObject, error) {
	c := ginCtx(ctx)
	if err := h.Auth.SendCode(c, req.Body.Email, verifycode.Purpose(req.Body.Purpose), meta(c)); err != nil {
		return nil, err
	}
	return pubapi.SendEmailCode204Response{}, nil
}

func (h *Handler) Register(ctx context.Context, req pubapi.RegisterRequestObject) (pubapi.RegisterResponseObject, error) {
	c := ginCtx(ctx)
	b := req.Body
	in := auth.RegisterInput{
		Email: b.Email, Code: b.Code, Password: b.Password, InviterID: b.InviterId, Client: clientOf(b.Client),
	}
	if b.Nickname != nil {
		in.Nickname = *b.Nickname
	}
	res, err := h.Auth.Register(c, in, meta(c))
	if err != nil {
		return nil, err
	}
	return pubapi.Register201JSONResponse(h.authResult(c, res, true)), nil
}

func (h *Handler) Login(ctx context.Context, req pubapi.LoginRequestObject) (pubapi.LoginResponseObject, error) {
	c := ginCtx(ctx)
	b := req.Body
	remember := b.Remember == nil || *b.Remember
	res, err := h.Auth.Login(c, auth.LoginInput{
		Email: b.Email, Password: b.Password, Remember: remember, Client: clientOf(b.Client),
	}, meta(c))
	if err != nil {
		return nil, err
	}
	return pubapi.Login200JSONResponse(h.authResult(c, res, remember)), nil
}

// authResult 网页写入会话 Cookie；App 在响应体中返回令牌。
func (h *Handler) authResult(c *gin.Context, res auth.Result, persistent bool) pubapi.AuthResult {
	out := pubapi.AuthResult{User: toUser(res.User)}
	if res.Session.Client == session.ClientApp {
		out.Token = &res.Token
		return out
	}
	var maxAge = res.Session.TTL
	if !persistent {
		maxAge = 0
	}
	h.Realm.Cookies.SetSession(c, res.Token, maxAge)
	return out
}

func (h *Handler) Logout(ctx context.Context, _ pubapi.LogoutRequestObject) (pubapi.LogoutResponseObject, error) {
	c := ginCtx(ctx)
	token, fromCookie := h.Realm.Token(c)
	if err := h.Auth.Logout(c, token); err != nil {
		return nil, err
	}
	if fromCookie {
		h.Realm.Cookies.ClearSession(c)
	}
	return pubapi.Logout204Response{}, nil
}

func (h *Handler) ResetPassword(ctx context.Context, req pubapi.ResetPasswordRequestObject) (pubapi.ResetPasswordResponseObject, error) {
	b := req.Body
	if err := h.Auth.ResetPassword(ctx, b.Email, b.Code, b.Password); err != nil {
		return nil, err
	}
	return pubapi.ResetPassword204Response{}, nil
}

func clientOf(c *pubapi.ClientType) session.Client {
	if c != nil && *c == pubapi.App {
		return session.ClientApp
	}
	return session.ClientWeb
}
