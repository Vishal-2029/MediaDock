package history

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

// Handler exposes HTTP endpoints for download history management.
type Handler struct {
	service *Service
}

// NewHandler constructs a history Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/history.
func (h *Handler) List(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	page := utils.ParsePagination(c)
	items, total, err := h.service.List(c.UserContext(), userID, page.Offset, page.PerPage)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "HISTORY_LIST_FAILED", "could not list download history")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, items, utils.NewPageMeta(page, total))
}

// GetByID handles GET /api/v1/history/:id.
func (h *Handler) GetByID(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid history ID")
	}

	rec, err := h.service.GetByID(c.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "HISTORY_NOT_FOUND", "history record not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "HISTORY_FETCH_FAILED", "could not load history record")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, rec)
}

// Delete handles DELETE /api/v1/history/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid history ID")
	}

	if err := h.service.Delete(c.UserContext(), id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "HISTORY_NOT_FOUND", "history record not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "HISTORY_DELETE_FAILED", "could not delete history record")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"deleted": true})
}

