package middleware_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"golearn/middleware"
	"golearn/session"
)

func newApp(sessions *session.Store) *fiber.App {
	app := fiber.New()
	app.Get("/admin", middleware.AdminAuth(sessions), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func request(t *testing.T, app *fiber.App, configure func(*http.Request)) int {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/admin", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if configure != nil {
		configure(req)
	}

	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return res.StatusCode
}

func TestAdminAuthRejectsWithoutSession(t *testing.T) {
	app := newApp(session.NewStore(0))

	cases := []struct {
		name      string
		configure func(*http.Request)
	}{
		{"no credentials", nil},
		{"unknown cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: middleware.AdminSessionCookie, Value: "nope"})
		}},
		{"unknown bearer token", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer nope")
		}},
		{"bearer prefix only", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer ")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := request(t, app, tc.configure); got != fiber.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", got, fiber.StatusUnauthorized)
			}
		})
	}
}

func TestAdminAuthNilStoreFailsClosed(t *testing.T) {
	app := newApp(nil)

	if got := request(t, app, nil); got != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", got, fiber.StatusUnauthorized)
	}
}

func TestAdminAuthAcceptsSessionCookie(t *testing.T) {
	sessions := session.NewStore(0)
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	app := newApp(sessions)

	if got := request(t, app, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: middleware.AdminSessionCookie, Value: token})
	}); got != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", got, fiber.StatusOK)
	}
}

func TestAdminAuthAcceptsBearerToken(t *testing.T) {
	sessions := session.NewStore(0)
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	app := newApp(sessions)

	if got := request(t, app, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
	}); got != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", got, fiber.StatusOK)
	}
}

func TestAdminAuthRejectsRevokedSession(t *testing.T) {
	sessions := session.NewStore(0)
	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sessions.Delete(token)

	app := newApp(sessions)

	if got := request(t, app, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: middleware.AdminSessionCookie, Value: token})
	}); got != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", got, fiber.StatusUnauthorized)
	}
}
