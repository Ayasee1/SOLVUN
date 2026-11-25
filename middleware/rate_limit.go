package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimits holds the per-client limits applied to public endpoints.
//
// A disabled set returns pass-through handlers, so the route table stays the
// same in development and under load tests.
type RateLimits struct {
	Enabled bool
}

// Handler allows max requests per window for a single client.
//
// Every call creates its own limiter, so buckets never leak between routes: a
// client can spend its order budget and still fetch products. The client key is
// the address Fiber resolves in c.IP(), which honours X-Forwarded-For when
// TRUST_PROXY is on.
func (l RateLimits) Handler(max int, window time.Duration) fiber.Handler {
	if !l.Enabled {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: window,
		// Sliding window avoids the burst a fixed window allows at the
		// boundary between two windows.
		LimiterMiddleware: limiter.SlidingWindow{},
		LimitReached: func(c *fiber.Ctx) error {
			// The limiter already set Retry-After before calling this.
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests, please slow down",
			})
		},
	})
}
