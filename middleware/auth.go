package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"golearn/session"
)

// AdminSessionCookie is the HttpOnly cookie that carries the admin session
// token. The token never reaches JavaScript, so it cannot leak through the
// frontend bundle.
const AdminSessionCookie = "solvun_admin_session"

// AdminAuth protects admin routes with a server-side session. It fails closed:
// a missing or unknown token is always rejected.
func AdminAuth(sessions *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if sessions == nil || !sessions.Valid(SessionToken(c)) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		return c.Next()
	}
}

// SessionToken reads the admin session token from the cookie, falling back to
// an Authorization: Bearer header for scripts and health probes.
func SessionToken(c *fiber.Ctx) string {
	if token := c.Cookies(AdminSessionCookie); token != "" {
		return token
	}

	if header := c.Get(fiber.HeaderAuthorization); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}

	return ""
}
