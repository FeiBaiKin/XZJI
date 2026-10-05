package public

import (
	"context"

	pubapi "xzji/internal/apigen/public"
)

func (h *Handler) GetSiteInfo(ctx context.Context, _ pubapi.GetSiteInfoRequestObject) (pubapi.GetSiteInfoResponseObject, error) {
	site, err := h.Site.Get(ctx)
	if err != nil {
		return nil, err
	}
	return pubapi.GetSiteInfo200JSONResponse{
		FooterAbout: site.FooterAbout, Icp: site.ICP, PoliceIcp: site.PoliceICP,
	}, nil
}

func (h *Handler) GetSiteDocuments(ctx context.Context, _ pubapi.GetSiteDocumentsRequestObject) (pubapi.GetSiteDocumentsResponseObject, error) {
	site, err := h.Site.Get(ctx)
	if err != nil {
		return nil, err
	}
	return pubapi.GetSiteDocuments200JSONResponse{Terms: site.TermsHTML, Privacy: site.PrivacyHTML}, nil
}
