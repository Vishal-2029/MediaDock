package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mediadock/backend/internal/cache"
	"github.com/mediadock/backend/internal/utils"
)

// RateLimit enforces a Redis-backed fixed-window request quota per client IP.
func RateLimit(redisCache *cache.RedisClient, maxRequests int, window time.Duration) fiber.Handler {
	if maxRequests <= 0 {
		maxRequests = 100
	}
	if window <= 0 {
		window = time.Minute
	}

	return func(c *fiber.Ctx) error {
		if redisCache == nil || redisCache.Client == nil {
			return c.Next()
		}

		ip := c.IP()
		bucket := time.Now().UTC().Unix() / int64(window.Seconds())
		key := fmt.Sprintf("ratelimit:%s:%d", ip, bucket)

		ctx := c.UserContext()
		count, err := redisCache.Client.Incr(ctx, key).Result()
		if err != nil {
			// Fail open on transient Redis issues rather than breaking API availability.
			return c.Next()
		}
		if count == 1 {
			_ = redisCache.Client.Expire(ctx, key, window).Err()
		}

		remaining := maxRequests - int(count)
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if int(count) > maxRequests {
			return utils.JSONError(c, fiber.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests, please try again later")
		}

		return c.Next()
	}
}

