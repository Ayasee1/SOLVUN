package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"golearn/config"
	"golearn/handler"
	"golearn/middleware"
	"golearn/session"
)

const (
	testAdminEmail    = "owner@solvun.test"
	testAdminPassword = "correct horse battery"
)

func newAdminApp(t *testing.T, cfg config.Config) (*fiber.App, *session.Store) {
	t.Helper()

	sessions := session.NewStore(0)
	auth := handler.NewAdminAuthHandler(cfg, sessions)

	app := fiber.New()
	app.Post("/admin/session", auth.Login)
	app.Get("/admin/session", auth.Session)
	app.Delete("/admin/session", auth.Logout)
	app.Get("/admin/protected", middleware.AdminAuth(sessions), func(c *fiber.Ctx) error {
		return c.SendString("secret")
	})

	return app, sessions
}

func testAdminConfig(t *testing.T) config.Config {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testAdminPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return config.Config{
		AdminEmail:             testAdminEmail,
		AdminPasswordHash:      string(hash),
		AdminSessionTTLMinutes: 60,
		AdminCookieSecure:      false,
	}
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any, configure func(*http.Request)) *http.Response {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if configure != nil {
		configure(req)
	}

	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	return res
}

func sessionCookie(res *http.Response) *http.Cookie {
	for _, cookie := range res.Cookies() {
		if cookie.Name == middleware.AdminSessionCookie && cookie.Value != "" {
			return cookie
		}
	}
	return nil
}

func TestAdminLoginSuccess(t *testing.T) {
	app, _ := newAdminApp(t, testAdminConfig(t))

	res := doJSON(t, app, http.MethodPost, "/admin/session", map[string]string{
		"email":    testAdminEmail,
		"password": testAdminPassword,
	}, nil)
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}

	cookie := sessionCookie(res)
	if cookie == nil {
		t.Fatal("login did not set a session cookie")
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}

	// The cookie must unlock the protected route.
	protected := doJSON(t, app, http.MethodGet, "/admin/protected", nil, func(r *http.Request) {
		r.AddCookie(cookie)
	})
	defer protected.Body.Close()

	if protected.StatusCode != fiber.StatusOK {
		t.Fatalf("protected status = %d, want %d", protected.StatusCode, fiber.StatusOK)
	}
}

func TestAdminLoginRejectsBadCredentials(t *testing.T) {
	app, _ := newAdminApp(t, testAdminConfig(t))

	cases := []struct {
		name string
		body map[string]string
	}{
		{"wrong password", map[string]string{"email": testAdminEmail, "password": "wrong-password"}},
		{"wrong email", map[string]string{"email": "someone@else.test", "password": testAdminPassword}},
		{"empty body", map[string]string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := doJSON(t, app, http.MethodPost, "/admin/session", tc.body, nil)
			defer res.Body.Close()

			if res.StatusCode != fiber.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusUnauthorized)
			}
			if cookie := sessionCookie(res); cookie != nil {
				t.Fatal("failed login must not set a session cookie")
			}
		})
	}
}

func TestAdminLoginDisabledWhenNotConfigured(t *testing.T) {
	app, _ := newAdminApp(t, config.Config{})

	res := doJSON(t, app, http.MethodPost, "/admin/session", map[string]string{
		"email":    testAdminEmail,
		"password": testAdminPassword,
	}, nil)
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusServiceUnavailable)
	}
}

func TestAdminLoginRateLimit(t *testing.T) {
	app, _ := newAdminApp(t, testAdminConfig(t))

	body := map[string]string{"email": testAdminEmail, "password": "wrong-password"}

	for i := range 5 {
		res := doJSON(t, app, http.MethodPost, "/admin/session", body, nil)
		res.Body.Close()
		if res.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want %d", i+1, res.StatusCode, fiber.StatusUnauthorized)
		}
	}

	blocked := doJSON(t, app, http.MethodPost, "/admin/session", body, nil)
	defer blocked.Body.Close()

	if blocked.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", blocked.StatusCode, fiber.StatusTooManyRequests)
	}
}

func TestAdminSessionStatus(t *testing.T) {
	app, sessions := newAdminApp(t, testAdminConfig(t))

	anonymous := doJSON(t, app, http.MethodGet, "/admin/session", nil, nil)
	anonymous.Body.Close()
	if anonymous.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", anonymous.StatusCode, fiber.StatusUnauthorized)
	}

	token, err := sessions.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	authenticated := doJSON(t, app, http.MethodGet, "/admin/session", nil, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: middleware.AdminSessionCookie, Value: token})
	})
	defer authenticated.Body.Close()

	if authenticated.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", authenticated.StatusCode, fiber.StatusOK)
	}

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(authenticated.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Email != testAdminEmail {
		t.Fatalf("email = %q, want %q", payload.Email, testAdminEmail)
	}
}

func TestAdminLogoutRevokesSession(t *testing.T) {
	app, _ := newAdminApp(t, testAdminConfig(t))

	login := doJSON(t, app, http.MethodPost, "/admin/session", map[string]string{
		"email":    testAdminEmail,
		"password": testAdminPassword,
	}, nil)
	login.Body.Close()

	cookie := sessionCookie(login)
	if cookie == nil {
		t.Fatal("login did not set a session cookie")
	}

	logout := doJSON(t, app, http.MethodDelete, "/admin/session", nil, func(r *http.Request) {
		r.AddCookie(cookie)
	})
	logout.Body.Close()

	if logout.StatusCode != fiber.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.StatusCode, fiber.StatusNoContent)
	}

	protected := doJSON(t, app, http.MethodGet, "/admin/protected", nil, func(r *http.Request) {
		r.AddCookie(cookie)
	})
	defer protected.Body.Close()

	if protected.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", protected.StatusCode, fiber.StatusUnauthorized)
	}
}
