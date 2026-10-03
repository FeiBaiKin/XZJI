package admin

import (
	"context"
	"errors"
	"io"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/httpapi/apierr"
	"xzji/internal/httpapi/authn"
	"xzji/internal/module/media"
	"xzji/internal/module/setting"
)

// UploadMedia 读取表单中名为 file 的部分，其他部分忽略。
func (h *Handler) UploadMedia(ctx context.Context, req adminapi.UploadMediaRequestObject) (adminapi.UploadMediaResponseObject, error) {
	id, _ := authn.FromContext(ctx)
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, media.ErrEmpty
		}
		if err != nil {
			_ = ginCtx(ctx).Error(err)
			return nil, apierr.BadRequest
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		m, err := h.Media.Upload(ctx, id.UserID, part)
		part.Close()
		if err != nil {
			return nil, err
		}
		return adminapi.UploadMedia201JSONResponse(*toMedia(&m)), nil
	}
}

func toHomeSettings(home setting.Home) adminapi.HomeSettings {
	out := adminapi.HomeSettings{Banners: make([]adminapi.BannerSetting, len(home.Banners)), Notice: home.Notice}
	for i, b := range home.Banners {
		out.Banners[i] = adminapi.BannerSetting{
			Tag: b.Tag, Title: b.Title, Subtitle: b.Subtitle, ButtonText: b.ButtonText, Link: b.Link, Image: toMedia(b.Image),
		}
	}
	return out
}

func (h *Handler) GetHomeSettings(ctx context.Context, _ adminapi.GetHomeSettingsRequestObject) (adminapi.GetHomeSettingsResponseObject, error) {
	home, err := h.Home.Get(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.GetHomeSettings200JSONResponse(toHomeSettings(home)), nil
}

func (h *Handler) UpdateHomeSettings(ctx context.Context, req adminapi.UpdateHomeSettingsRequestObject) (adminapi.UpdateHomeSettingsResponseObject, error) {
	in := setting.Home{Banners: make([]setting.Banner, len(req.Body.Banners)), Notice: req.Body.Notice}
	for i, b := range req.Body.Banners {
		in.Banners[i] = setting.Banner{
			Tag: deref(b.Tag), Title: b.Title, Subtitle: deref(b.Subtitle), ButtonText: deref(b.ButtonText),
			Link: b.Link, ImageID: deref(b.ImageId),
		}
	}
	home, err := h.Home.Set(ctx, in)
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateHomeSettings200JSONResponse(toHomeSettings(home)), nil
}
