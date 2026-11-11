package conversion

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/modules/processing"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

// Handler exposes HTTP endpoints for media conversion jobs.
type Handler struct {
	service   *Service
	validator *utils.Validator
}

// NewHandler constructs a conversion Handler.
func NewHandler(service *Service, validator *utils.Validator) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
	}
}

// Create handles POST /api/v1/conversions (multipart/form-data).
func (h *Handler) Create(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "MISSING_FILE", "multipart form field 'file' is required")
	}

	req := CreateConversionRequest{
		TargetFormat:     strings.TrimSpace(c.FormValue("target_format")),
		TargetResolution: strings.TrimSpace(c.FormValue("target_resolution")),
		Compress:         strings.EqualFold(c.FormValue("compress"), "true") || c.FormValue("compress") == "1",
		ExtractAudio:     strings.EqualFold(c.FormValue("extract_audio"), "true") || c.FormValue("extract_audio") == "1",
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	f, err := fileHeader.Open()
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "FILE_OPEN_FAILED", "could not read uploaded file")
	}
	defer f.Close()

	job, err := h.service.CreateFromFile(c.UserContext(), userID, fileHeader.Filename, f, req)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrFileTooLarge):
			return utils.JSONError(c, fiber.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", err.Error())
		case errors.Is(err, utils.ErrUnsupportedFileType), errors.Is(err, processing.ErrUnsupportedTargetFormat):
			return utils.JSONError(c, fiber.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA", err.Error())
		default:
			return utils.JSONError(c, fiber.StatusInternalServerError, "CONVERSION_CREATE_FAILED", "could not create conversion job")
		}
	}

	return utils.JSONSuccess(c, fiber.StatusAccepted, job)
}

// GetByID handles GET /api/v1/conversions/:id.
func (h *Handler) GetByID(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid conversion job ID")
	}

	job, err := h.service.GetJob(c.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "CONVERSION_NOT_FOUND", "conversion job not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "CONVERSION_FETCH_FAILED", "could not load conversion job")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, job)
}

// GetStatus handles GET /api/v1/conversions/:id/status.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid conversion job ID")
	}

	status, err := h.service.GetStatus(c.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "CONVERSION_NOT_FOUND", "conversion job not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "STATUS_FETCH_FAILED", "could not load conversion status")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, status)
}

// GetFile handles GET /api/v1/conversions/:id/file.
func (h *Handler) GetFile(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid conversion job ID")
	}

	reader, size, fileName, err := h.service.OpenOutput(c.UserContext(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return utils.JSONError(c, fiber.StatusNotFound, "CONVERSION_NOT_FOUND", "conversion job not found")
		case errors.Is(err, ErrConversionExpired):
			return utils.JSONError(c, fiber.StatusGone, "CONVERSION_EXPIRED", err.Error())
		case errors.Is(err, ErrConversionNotReady):
			return utils.JSONError(c, fiber.StatusConflict, "CONVERSION_NOT_READY", err.Error())
		default:
			return utils.JSONError(c, fiber.StatusInternalServerError, "CONVERSION_DOWNLOAD_FAILED", "could not retrieve converted file")
		}
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Type", "application/octet-stream")
	return c.SendStream(reader, int(size))
}

