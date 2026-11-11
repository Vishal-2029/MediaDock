package media

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/modules/providers"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

// Handler exposes HTTP endpoints for media inspection and metadata lookup.
type Handler struct {
	service   *Service
	validator *utils.Validator
}

// NewHandler constructs a media Handler.
func NewHandler(service *Service, validator *utils.Validator) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
	}
}

// Inspect handles POST /api/v1/media/inspect.
func (h *Handler) Inspect(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	var req InspectRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if req.AccessToken == "" {
		req.AccessToken = strings.TrimSpace(c.Get("X-Platform-Access-Token"))
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	resp, err := h.service.Inspect(c.UserContext(), userID, req)
	if err != nil {
		var provErr *providers.ProviderError
		if errors.As(err, &provErr) {
			details := map[string]string{}
			if provErr.Alternative != "" {
				details["alternative"] = provErr.Alternative
			}
			if provErr.Provider != "" {
				details["provider"] = string(provErr.Provider)
			}
			status := provErr.HTTPStatus
			if status == 0 {
				status = fiber.StatusBadRequest
			}
			return utils.JSONError(c, status, provErr.Code, provErr.Message, details)
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "INSPECT_FAILED", "failed to inspect media URL")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, resp)
}

// GetByID handles GET /api/v1/media/:id.
func (h *Handler) GetByID(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	mediaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid media ID")
	}

	resp, err := h.service.GetByID(c.UserContext(), mediaID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "MEDIA_NOT_FOUND", "media item not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "MEDIA_FETCH_FAILED", "failed to retrieve media item")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, resp)
}

// List handles GET /api/v1/media.
func (h *Handler) List(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	page := utils.ParsePagination(c)
	items, total, err := h.service.List(c.UserContext(), userID, page.Offset, page.PerPage)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "MEDIA_LIST_FAILED", "could not list media items")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, items, utils.NewPageMeta(page, total))
}

// Delete handles DELETE /api/v1/media/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	mediaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid media ID")
	}

	if err := h.service.Delete(c.UserContext(), mediaID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "MEDIA_NOT_FOUND", "media item not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "MEDIA_DELETE_FAILED", "could not delete media item")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"deleted": true})
}

