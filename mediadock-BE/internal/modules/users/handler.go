package users

import (
	"github.com/gofiber/fiber/v2"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/utils"
)

// Handler exposes HTTP endpoints for user settings.
type Handler struct {
	service   *Service
	validator *utils.Validator
}

// NewHandler constructs a users Handler.
func NewHandler(service *Service, validator *utils.Validator) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
	}
}

// GetSettings handles GET /api/v1/settings.
func (h *Handler) GetSettings(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	settings, err := h.service.GetSettings(c.UserContext(), userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "SETTINGS_LOAD_FAILED", "could not retrieve settings")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, settings)
}

// UpdateSettings handles PUT /api/v1/settings.
func (h *Handler) UpdateSettings(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	settings, err := h.service.UpdateSettings(c.UserContext(), userID, req)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "SETTINGS_UPDATE_FAILED", "could not update settings")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, settings)
}

