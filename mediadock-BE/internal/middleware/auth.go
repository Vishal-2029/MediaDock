package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/modules/auth"
	"github.com/mediadock/backend/internal/utils"
)

// RequireAuth verifies the JWT access token from the Authorization Bearer header
// or the HttpOnly access cookie, and checks Redis revocation.
func RequireAuth(authService *auth.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var token string
		header := strings.TrimSpace(c.Get("Authorization"))
		if header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_AUTH_HEADER", "authorization header must use Bearer scheme")
			}
			token = strings.TrimSpace(parts[1])
		} else {
			token = strings.TrimSpace(c.Cookies(auth.AccessCookieName))
		}

		if token == "" {
			return utils.JSONError(c, fiber.StatusUnauthorized, "MISSING_TOKEN", "authentication is required")
		}

		claims, userID, err := authService.ParseAndValidateAccess(c.UserContext(), token)
		if err != nil {
			return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "invalid, expired, or revoked access token")
		}

		c.Locals(auth.ContextUserIDKey, userID)
		c.Locals(auth.ContextClaimsKey, claims)
		return c.Next()
	}
}

// OptionalAuthOrGuest resolves an authenticated JWT user when supplied, or provisions
// the login-free guest workspace so media tools work immediately without requiring sign-in.
func OptionalAuthOrGuest(authService *auth.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		if header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_AUTH_HEADER", "authorization header must use Bearer scheme")
			}
			claims, userID, err := authService.ParseAndValidateAccess(c.UserContext(), strings.TrimSpace(parts[1]))
			if err != nil {
				return utils.JSONError(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "invalid, expired, or revoked access token")
			}
			c.Locals(auth.ContextUserIDKey, userID)
			c.Locals(auth.ContextClaimsKey, claims)
			return c.Next()
		}

		if cookieToken := strings.TrimSpace(c.Cookies(auth.AccessCookieName)); cookieToken != "" {
			if claims, userID, err := authService.ParseAndValidateAccess(c.UserContext(), cookieToken); err == nil {
				c.Locals(auth.ContextUserIDKey, userID)
				c.Locals(auth.ContextClaimsKey, claims)
				return c.Next()
			}
		}

		guestID := auth.DefaultGuestUserID
		if rawGuest := strings.TrimSpace(c.Cookies(auth.GuestCookieName)); rawGuest != "" {
			if parsed, err := uuid.Parse(rawGuest); err == nil && parsed != uuid.Nil {
				guestID = parsed
			}
		}

		if err := authService.EnsureGuestWorkspace(c.UserContext(), guestID); err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, "WORKSPACE_INIT_FAILED", "could not initialize guest workspace")
		}

		c.Locals(auth.ContextUserIDKey, guestID)
		return c.Next()
	}
}
