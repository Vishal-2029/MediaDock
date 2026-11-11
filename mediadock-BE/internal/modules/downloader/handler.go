package downloader

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/modules/storage"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

// Handler exposes HTTP endpoints for download jobs.
type Handler struct {
	service   *Service
	validator *utils.Validator
}

// NewHandler constructs a downloader Handler.
func NewHandler(service *Service, validator *utils.Validator) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
	}
}

// Create handles POST /api/v1/downloads.
func (h *Handler) Create(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	var req CreateDownloadRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if req.AccessToken == "" {
		req.AccessToken = strings.TrimSpace(c.Get("X-Platform-Access-Token"))
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	job, err := h.service.CreateJob(c.UserContext(), userID, req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "MEDIA_NOT_FOUND", "inspected media item not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "DOWNLOAD_CREATE_FAILED", "could not create download job")
	}

	return utils.JSONSuccess(c, fiber.StatusAccepted, job)
}

// GetByID handles GET /api/v1/downloads/:id.
func (h *Handler) GetByID(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid download job ID")
	}

	job, err := h.service.GetJob(c.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "JOB_NOT_FOUND", "download job not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "JOB_FETCH_FAILED", "could not load download job")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, job)
}

// List handles GET /api/v1/downloads.
func (h *Handler) List(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	page := utils.ParsePagination(c)
	jobs, total, err := h.service.ListJobs(c.UserContext(), userID, page.Offset, page.PerPage)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "JOBS_LIST_FAILED", "could not list download jobs")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, jobs, utils.NewPageMeta(page, total))
}

// GetStatus handles GET /api/v1/downloads/:id/status.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid download job ID")
	}

	status, err := h.service.GetStatus(c.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "JOB_NOT_FOUND", "download job not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "STATUS_FETCH_FAILED", "could not load job status")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, status)
}

// GetFile handles GET /api/v1/downloads/:id/file.
func (h *Handler) GetFile(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid download job ID")
	}

	res, err := h.service.OpenFile(c.UserContext(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound), errors.Is(err, storage.ErrObjectNotFound):
			return utils.JSONError(c, fiber.StatusNotFound, "FILE_NOT_FOUND", "download artifact not found")
		case errors.Is(err, ErrJobExpired):
			return utils.JSONError(c, fiber.StatusGone, "JOB_EXPIRED", err.Error())
		case errors.Is(err, ErrJobNotReady):
			return utils.JSONError(c, fiber.StatusConflict, "JOB_NOT_READY", err.Error())
		default:
			return utils.JSONError(c, fiber.StatusInternalServerError, "FILE_DOWNLOAD_FAILED", "could not retrieve download file")
		}
	}

	if res.SignedURL != "" {
		return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{
			"file_name":    res.FileName,
			"download_url": res.SignedURL,
		})
	}

	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, res.FileName))
	c.Set("Content-Type", "application/octet-stream")
	return c.SendStream(res.Reader, int(res.Size))
}

// Delete handles DELETE /api/v1/downloads/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_ID", "invalid download job ID")
	}

	if err := h.service.DeleteJob(c.UserContext(), id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "JOB_NOT_FOUND", "download job not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "JOB_DELETE_FAILED", "could not delete download job")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"deleted": true})
}

