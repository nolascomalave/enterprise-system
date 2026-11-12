package http

import (
	"context"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"github.com/gin-gonic/gin"
)

type CatalogUseCase interface {
	ListIdentityDocumentTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error)
	ListRoleTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error)
	ListDepartments(ctx context.Context, locale string) ([]*domain.LocalizedDepartment, error)
}

type CatalogHandler struct {
	uc CatalogUseCase
}

func NewCatalogHandler(uc CatalogUseCase) *CatalogHandler {
	return &CatalogHandler{uc: uc}
}

// ListIdentityDocumentTypes godoc
// @Summary      List identity document types
// @Description  Returns the global catalog localized by Accept-Language with fallback to the default locale and then to the code.
// @Tags         Catalogs
// @Produce      json
// @Param        Accept-Language header string false "Locale BCP 47 (es, en, pt-BR)"
// @Success      200 {object} dto.CatalogListEnvelope
// @Failure      500 {object} dto.ErrorResponse
// @Router       /identity-document-types [get]
func (h *CatalogHandler) ListIdentityDocumentTypes(c *gin.Context) {
	entries, err := h.uc.ListIdentityDocumentTypes(c.Request.Context(), LocaleFromContext(c.Request.Context()))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.CatalogListEnvelope{Data: dto.NewLocalizedCatalogListResponse(entries)})
}

// ListRoleTypes godoc
// @Summary      List role types
// @Description  Returns the global role type catalog localized by Accept-Language with fallback to the default locale and then to the code.
// @Tags         Catalogs
// @Produce      json
// @Param        Accept-Language header string false "Locale BCP 47 (es, en, pt-BR)"
// @Success      200 {object} dto.CatalogListEnvelope
// @Failure      500 {object} dto.ErrorResponse
// @Router       /role-types [get]
func (h *CatalogHandler) ListRoleTypes(c *gin.Context) {
	entries, err := h.uc.ListRoleTypes(c.Request.Context(), LocaleFromContext(c.Request.Context()))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.CatalogListEnvelope{Data: dto.NewLocalizedCatalogListResponse(entries)})
}
