package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS configures Cross-Origin Resource Sharing from ALLOWED_ORIGINS.
func CORS(allowedOrigins []string) fiber.Handler {
	origins := "*"
	allowCredentials := false
	if len(allowedOrigins) > 0 {
		origins = strings.Join(allowedOrigins, ",")
		if origins != "*" {
			allowCredentials = true
		}
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID,X-Platform-Access-Token",
		ExposeHeaders:    "Content-Disposition,X-Request-ID,X-RateLimit-Limit,X-RateLimit-Remaining",
		AllowCredentials: allowCredentials,
		MaxAge:           3600,
	})
}

