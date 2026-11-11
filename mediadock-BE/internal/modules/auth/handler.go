package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/repository"
	"github.com/mediadock/backend/internal/utils"
)

const (
	ContextUserIDKey  = "user_id"
	ContextClaimsKey  = "token_claims"
	AccessCookieName  = "mediadock_access"
	RefreshCookieName = "mediadock_refresh"
)

// Handler exposes HTTP endpoints for authentication.
type Handler struct {
	service   *Service
	validator *utils.Validator
}

// NewHandler constructs an auth Handler.
func NewHandler(service *Service, validator *utils.Validator) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
	}
}

func (h *Handler) setAuthCookies(c *fiber.Ctx, tokens TokenPair) {
	accessTTL, refreshTTL, secure := h.service.CookieSettings()
	now := time.Now().UTC()

	c.Cookie(&fiber.Cookie{
		Name:     AccessCookieName,
		Value:    tokens.AccessToken,
		Path:     "/",
		Expires:  now.Add(accessTTL),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	c.Cookie(&fiber.Cookie{
		Name:     RefreshCookieName,
		Value:    tokens.RefreshToken,
		Path:     "/api/v1/auth",
		Expires:  now.Add(refreshTTL),
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

func (h *Handler) clearAuthCookies(c *fiber.Ctx) {
	_, _, secure := h.service.CookieSettings()
	past := time.Unix(0, 0).UTC()

	c.Cookie(&fiber.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		Expires:  past,
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	c.Cookie(&fiber.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		Expires:  past,
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// Register handles POST /api/v1/auth/register.
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	resp, err := h.service.Register(c.UserContext(), req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return utils.JSONError(c, fiber.StatusConflict, "EMAIL_TAKEN", err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "REGISTER_FAILED", "could not register user")
	}

	h.setAuthCookies(c, resp.Tokens)
	return utils.JSONSuccess(c, fiber.StatusCreated, resp)
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	resp, err := h.service.Login(c.UserContext(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "LOGIN_FAILED", "authentication failed")
	}

	h.setAuthCookies(c, resp.Tokens)
	return utils.JSONSuccess(c, fiber.StatusOK, resp)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		}
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		req.RefreshToken = strings.TrimSpace(c.Cookies(RefreshCookieName))
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	tokens, err := h.service.Refresh(c.UserContext(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrTokenRevoked) {
			h.clearAuthCookies(c)
			return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "REFRESH_FAILED", "could not refresh token")
	}

	h.setAuthCookies(c, *tokens)
	return utils.JSONSuccess(c, fiber.StatusOK, tokens)
}

// Logout handles POST /api/v1/auth/logout.
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if len(c.Body()) > 0 {
		_ = c.BodyParser(&req)
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		req.RefreshToken = strings.TrimSpace(c.Cookies(RefreshCookieName))
	}

	claims, _ := c.Locals(ContextClaimsKey).(*TokenClaims)
	if err := h.service.Logout(c.UserContext(), claims, req.RefreshToken); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "LOGOUT_FAILED", "could not complete logout")
	}

	h.clearAuthCookies(c)
	return utils.JSONSuccess(c, fiber.StatusOK, fiber.Map{"message": "logged out successfully"})
}

// Me handles GET /api/v1/auth/me.
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, ok := c.Locals(ContextUserIDKey).(uuid.UUID)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	profile, err := h.service.GetMe(c.UserContext(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "USER_NOT_FOUND", "user not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "PROFILE_FAILED", "could not retrieve user profile")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, profile)
}

// UpdateMe handles PUT /api/v1/auth/me.
func (h *Handler) UpdateMe(c *fiber.Ctx) error {
	userID, ok := c.Locals(ContextUserIDKey).(uuid.UUID)
	if !ok {
		return utils.JSONError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
	}
	if errs := h.validator.Validate(req); errs != nil {
		return utils.JSONError(c, fiber.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", errs)
	}

	profile, err := h.service.UpdateProfile(c.UserContext(), userID, req.Name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, "USER_NOT_FOUND", "user not found")
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "PROFILE_UPDATE_FAILED", "could not update user profile")
	}

	return utils.JSONSuccess(c, fiber.StatusOK, profile)
}

// UserIDFromContext extracts the authenticated user's UUID from Fiber locals.
func UserIDFromContext(c *fiber.Ctx) (uuid.UUID, bool) {
	id, ok := c.Locals(ContextUserIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
