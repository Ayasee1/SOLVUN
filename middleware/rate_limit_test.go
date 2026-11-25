package middleware_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"golearn/middleware"
)

func newLimitedApp(limits middleware.RateLimits, max int, window time.Duration) *fiber.App {
	// app.Test() serves requests over a fake connection, so every request would
	// look like the same peer. Trusting X-Forwarded-For mirrors production,
	// where nginx forwards the real address and TRUST_PROXY is enabled.
	app := fiber.New(fiber.Config{ProxyHeader: fiber.HeaderXForwardedFor})
	app.Get("/limited", limits.Handler(max, window), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func requestAs(t *testing.T, app *fiber.App, ip string) (*http.Response, string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/limited", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(fiber.HeaderXForwardedFor, ip)

	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	res.Body.Close()

	return res, string(body)
}

func TestRateLimitAllowsUpToMax(t *testing.T) {
	app := newLimitedApp(middleware.RateLimits{Enabled: true}, 2, time.Minute)

	for i := range 2 {
		res, body := requestAs(t, app, "10.0.0.1")
		if res.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, res.StatusCode, fiber.StatusOK)
		}
		if body != "ok" {
			t.Fatalf("request %d: body = %q", i+1, body)
		}
	}

	res, body := requestAs(t, app, "10.0.0.1")
	if res.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusTooManyRequests)
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("429 body is not JSON: %v (%q)", err, body)
	}
	if !strings.Contains(strings.ToLower(payload.Error), "too many requests") {
		t.Fatalf("unexpected error message %q", payload.Error)
	}
	if res.Header.Get(fiber.HeaderRetryAfter) == "" {
		t.Fatal("429 response is missing Retry-After")
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	app := newLimitedApp(middleware.RateLimits{Enabled: true}, 1, time.Minute)

	if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusOK {
		t.Fatalf("first client: status = %d", res.StatusCode)
	}
	if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("first client should be limited, status = %d", res.StatusCode)
	}

	// A different address keeps its own bucket.
	if res, _ := requestAs(t, app, "10.0.0.2"); res.StatusCode != fiber.StatusOK {
		t.Fatalf("second client: status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
}

func TestRateLimitDisabled(t *testing.T) {
	app := newLimitedApp(middleware.RateLimits{Enabled: false}, 1, time.Minute)

	for i := range 10 {
		if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, res.StatusCode, fiber.StatusOK)
		}
	}
}

func TestRateLimitWindowResets(t *testing.T) {
	// Fiber's limiter tracks windows in whole seconds, so one second is the
	// smallest meaningful window.
	window := time.Second
	app := newLimitedApp(middleware.RateLimits{Enabled: true}, 1, window)

	if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusOK {
		t.Fatalf("first request: status = %d", res.StatusCode)
	}
	if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", res.StatusCode)
	}

	// Two full windows guarantee the sliding window is fully reset.
	time.Sleep(2*window + 200*time.Millisecond)

	if res, _ := requestAs(t, app, "10.0.0.1"); res.StatusCode != fiber.StatusOK {
		t.Fatalf("after the window: status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
}
