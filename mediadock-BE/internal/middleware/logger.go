package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const ContextRequestIDKey = "request_id"

// RequestLogger assigns a correlation ID and emits structured access logs without sensitive headers.
func RequestLogger(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Set("X-Request-ID", reqID)
		c.Locals(ContextRequestIDKey, reqID)

		err := c.Next()

		status := c.Response().StatusCode()
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			} else if status == fiber.StatusOK {
				status = fiber.StatusInternalServerError
			}
		}

		log.Info("http_request",
			"request_id", reqID,
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)

		return err
	}
}

