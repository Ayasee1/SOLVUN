package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
)

// AdminAuth protects admin routes with a static API key sent in the X-API-Key
// header. It fails closed: if no key is configured, every request is rejected.
func AdminAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		provided := c.Get("X-API-Key")

		if apiKey == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		return c.Next()
	}
}
