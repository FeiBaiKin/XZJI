package admin

import (
	"context"

	adminapi "xzji/internal/apigen/admin"
	"xzji/internal/module/setting"
)

func toSiteSettings(site setting.Site) adminapi.SiteSettings {
	return adminapi.SiteSettings{
		FooterAbout: site.FooterAbout, Icp: site.ICP, PoliceIcp: site.PoliceICP,
		TermsHtml: site.TermsHTML, PrivacyHtml: site.PrivacyHTML,
	}
}

func (h *Handler) GetSiteSettings(ctx context.Context, _ adminapi.GetSiteSettingsRequestObject) (adminapi.GetSiteSettingsResponseObject, error) {
	site, err := h.Site.Get(ctx)
	if err != nil {
		return nil, err
	}
	return adminapi.GetSiteSettings200JSONResponse(toSiteSettings(site)), nil
}

func (h *Handler) UpdateSiteSettings(ctx context.Context, req adminapi.UpdateSiteSettingsRequestObject) (adminapi.UpdateSiteSettingsResponseObject, error) {
	site, err := h.Site.Set(ctx, setting.Site{
		FooterAbout: req.Body.FooterAbout, ICP: req.Body.Icp, PoliceICP: req.Body.PoliceIcp,
		TermsHTML: req.Body.TermsHtml, PrivacyHTML: req.Body.PrivacyHtml,
	})
	if err != nil {
		return nil, err
	}
	return adminapi.UpdateSiteSettings200JSONResponse(toSiteSettings(site)), nil
}
