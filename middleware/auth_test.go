package middleware_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"golearn/middleware"
)

func TestAdminAuth(t *testing.T) {
	const key = "secret-key"

	newApp := func(apiKey string) *fiber.App {
		app := fiber.New()
		app.Get("/admin", middleware.AdminAuth(apiKey), func(c *fiber.Ctx) error {
			return c.SendString("ok")
		})
		return app
	}

	cases := []struct {
		name     string
		apiKey   string
		header   string
		wantCode int
	}{
		{"valid key", key, key, fiber.StatusOK},
		{"wrong key", key, "nope", fiber.StatusUnauthorized},
		{"missing header", key, "", fiber.StatusUnauthorized},
		{"no key configured fails closed", "", "", fiber.StatusUnauthorized},
		{"no key configured rejects any key", "", key, fiber.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
			if tc.header != "" {
				req.Header.Set("X-API-Key", tc.header)
			}

			resp, err := newApp(tc.apiKey).Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.wantCode {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantCode)
			}
		})
	}
}
