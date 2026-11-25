package handler

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"golearn/config"
	"golearn/middleware"
	"golearn/session"
)

// AdminAuthHandler signs the single operator in with the credentials configured
// through the environment. The resulting session token lives in an HttpOnly
// cookie, so it is never exposed to JavaScript.
type AdminAuthHandler struct {
	email        string
	passwordHash []byte
	sessions     *session.Store
	ttl          time.Duration
	secureCookie bool
	limiter      *loginLimiter
}

// NewAdminAuthHandler builds the handler from configuration.
func NewAdminAuthHandler(cfg config.Config, sessions *session.Store) *AdminAuthHandler {
	return &AdminAuthHandler{
		email:        strings.ToLower(strings.TrimSpace(cfg.AdminEmail)),
		passwordHash: []byte(cfg.AdminPasswordHash),
		sessions:     sessions,
		ttl:          time.Duration(cfg.AdminSessionTTLMinutes) * time.Minute,
		secureCookie: cfg.AdminCookieSecure,
		limiter:      newLoginLimiter(5, 15*time.Minute),
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /admin/session.
func (h *AdminAuthHandler) Login(c *fiber.Ctx) error {
	if h.email == "" || len(h.passwordHash) == 0 || h.sessions == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Admin sign-in is not configured",
		})
	}

	ip := c.IP()
	if !h.limiter.allow(ip) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "Too many sign-in attempts, try again later",
		})
	}

	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	emailMatches := strings.ToLower(strings.TrimSpace(req.Email)) == h.email
	passwordMatches := bcrypt.CompareHashAndPassword(h.passwordHash, []byte(req.Password)) == nil

	// Compare both parts before branching so a wrong email and a wrong password
	// cost the same.
	if !emailMatches || !passwordMatches {
		h.limiter.fail(ip)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	h.limiter.reset(ip)

	token, err := h.sessions.Create()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not create a session",
		})
	}

	h.writeCookie(c, token, h.ttl)

	return c.JSON(fiber.Map{
		"email":      h.email,
		"expires_in": int(h.ttl.Seconds()),
	})
}

// Logout handles DELETE /admin/session.
func (h *AdminAuthHandler) Logout(c *fiber.Ctx) error {
	if h.sessions != nil {
		h.sessions.Delete(middleware.SessionToken(c))
	}

	h.writeCookie(c, "", -time.Hour)

	return c.SendStatus(fiber.StatusNoContent)
}

// Session handles GET /admin/session and lets the frontend check whether the
// cookie is still valid.
func (h *AdminAuthHandler) Session(c *fiber.Ctx) error {
	if h.sessions == nil || !h.sessions.Valid(middleware.SessionToken(c)) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	return c.JSON(fiber.Map{"email": h.email})
}

func (h *AdminAuthHandler) writeCookie(c *fiber.Ctx, value string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     middleware.AdminSessionCookie,
		Value:    value,
		Path:     "/",
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: fiber.CookieSameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
}

// loginLimiter blocks repeated failures from the same client. It is per-process
// and resets on restart, which is enough for a single-operator admin.
type loginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	attempts map[string]*attemptWindow
	now      func() time.Time
}

type attemptWindow struct {
	count int
	reset time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		max:      max,
		window:   window,
		attempts: make(map[string]*attemptWindow),
		now:      time.Now,
	}
}

// allow reports whether client may attempt a sign-in.
func (l *loginLimiter) allow(client string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	window, ok := l.attempts[client]
	if !ok || !now.Before(window.reset) {
		if ok {
			delete(l.attempts, client)
		}
		return true
	}

	return window.count < l.max
}

// fail records a failed attempt for client.
func (l *loginLimiter) fail(client string) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	window, ok := l.attempts[client]
	if !ok || !now.Before(window.reset) {
		l.attempts[client] = &attemptWindow{count: 1, reset: now.Add(l.window)}
		return
	}

	window.count++
}

// reset clears the counter after a successful sign-in.
func (l *loginLimiter) reset(client string) {
	l.mu.Lock()
	delete(l.attempts, client)
	l.mu.Unlock()
}
