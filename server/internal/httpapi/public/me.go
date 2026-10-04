package public

import (
	"context"

	pubapi "xzji/internal/apigen/public"
	"xzji/internal/httpapi/authn"
)

// 以下接口在契约中声明了 security，未登录的请求已被拦截，身份一定存在。

func (h *Handler) GetMe(ctx context.Context, _ pubapi.GetMeRequestObject) (pubapi.GetMeResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	u, err := h.Users.Get(ctx, id.UserID)
	if err != nil {
		return nil, err
	}
	return pubapi.GetMe200JSONResponse(h.toUser(ctx, u)), nil
}

func (h *Handler) UpdateMe(ctx context.Context, req pubapi.UpdateMeRequestObject) (pubapi.UpdateMeResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	u, err := h.Users.UpdateNickname(ctx, id.UserID, req.Body.Nickname)
	if err != nil {
		return nil, err
	}
	return pubapi.UpdateMe200JSONResponse(h.toUser(ctx, u)), nil
}

func (h *Handler) ChangePassword(ctx context.Context, req pubapi.ChangePasswordRequestObject) (pubapi.ChangePasswordResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	if err := h.Users.ChangePassword(ctx, id.UserID, req.Body.OldPassword, req.Body.NewPassword, id.SessionID); err != nil {
		return nil, err
	}
	return pubapi.ChangePassword204Response{}, nil
}
