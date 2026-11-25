package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	DatabaseURL       string
	CatalogDir        string
	MidtransServerKey string
	MidtransClientKey string
	MidtransEnv       string
	// email. Resend wins when set, then SMTP, otherwise emails are only logged.
	ResendAPIKey string
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPass     string
	MailFrom     string
	// shipping: provider is "biteship", "flat" or "disabled".
	ShippingProvider      string
	ShippingFlatRates     string
	BiteshipAPIKey        string
	BiteshipBaseURL       string
	StoreOriginPostalCode string
	StoreOriginLat        float64
	StoreOriginLon        float64
	StoreOriginContact    string
	StoreOriginPhone      string
	StoreOriginAddress    string
	StoreCollectionMethod string
	// StoreBaseURL is the public base URL of the store, used for payment
	// redirects and links inside emails.
	StoreBaseURL string
	// CORSOrigins is a comma-separated list of allowed browser origins.
	// Empty means "*" (development only).
	CORSOrigins string
	// MidtransExpiryMinutes shortens the Snap payment window. 0 keeps the
	// Midtrans default (24 hours); a small value is handy to test expiry.
	MidtransExpiryMinutes int
	// OrderPendingTTLMinutes is how long an unpaid order keeps its stock
	// reserved before the sweeper expires it.
	OrderPendingTTLMinutes int
	// OrderSweepIntervalSeconds is how often the sweeper looks for stale orders.
	OrderSweepIntervalSeconds int
	// AdminEmail and AdminPasswordHash are the single operator credentials for
	// /admin. AdminPasswordHash must be a bcrypt hash; see cmd/hashpw.
	AdminEmail        string
	AdminPasswordHash string
	// AdminSessionTTLMinutes is how long an admin session cookie stays valid.
	AdminSessionTTLMinutes int
	// AdminCookieSecure marks the session cookie Secure. Keep it true in
	// production; browsers still accept Secure cookies on http://localhost.
	AdminCookieSecure bool
}

// Load reads configuration from the environment, falling back to safe defaults.
func Load() Config {
	return Config{
		Port:                  getEnv("PORT", "3000"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		CatalogDir:            getEnv("CATALOG_DIR", "catalog"),
		MidtransServerKey:     os.Getenv("MIDTRANS_SERVER_KEY"),
		MidtransClientKey:     os.Getenv("MIDTRANS_CLIENT_KEY"),
		MidtransEnv:           getEnv("MIDTRANS_ENV", "sandbox"),
		MidtransExpiryMinutes: getEnvInt("MIDTRANS_EXPIRY_MINUTES", 0),

		OrderPendingTTLMinutes:    getEnvInt("ORDER_PENDING_TTL_MINUTES", 20),
		OrderSweepIntervalSeconds: getEnvInt("ORDER_SWEEP_INTERVAL_SECONDS", 60),

		AdminEmail:             strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		AdminPasswordHash:      strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH")),
		AdminSessionTTLMinutes: getEnvInt("ADMIN_SESSION_TTL_MINUTES", 720),
		AdminCookieSecure:      getEnvBool("ADMIN_COOKIE_SECURE", true),

		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPass:     os.Getenv("SMTP_PASS"),
		MailFrom:     getEnv("MAIL_FROM", "Solvun <no-reply@solvun.local>"),

		ShippingProvider:      os.Getenv("SHIPPING_PROVIDER"),
		ShippingFlatRates:     os.Getenv("SHIPPING_FLAT_RATES"),
		BiteshipAPIKey:        os.Getenv("BITESHIP_API_KEY"),
		BiteshipBaseURL:       getEnv("BITESHIP_BASE_URL", "https://api.biteship.com"),
		StoreOriginPostalCode: os.Getenv("STORE_ORIGIN_POSTAL_CODE"),
		StoreOriginLat:        getEnvFloat("STORE_ORIGIN_LAT", 0),
		StoreOriginLon:        getEnvFloat("STORE_ORIGIN_LON", 0),
		StoreOriginContact:    getEnv("STORE_ORIGIN_CONTACT", "Solvun"),
		StoreOriginPhone:      getEnv("STORE_ORIGIN_PHONE", ""),
		StoreOriginAddress:    getEnv("STORE_ORIGIN_ADDRESS", ""),
		StoreCollectionMethod: getEnv("STORE_COLLECTION_METHOD", "pickup"),
		StoreBaseURL:          getEnv("STORE_BASE_URL", "http://localhost:3000"),
		CORSOrigins:           os.Getenv("CORS_ORIGINS"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// AllowedOrigins returns the configured CORS origins, or ["*"] when unset.
func (c Config) AllowedOrigins() []string {
	if strings.TrimSpace(c.CORSOrigins) == "" {
		return []string{"*"}
	}
	parts := strings.Split(c.CORSOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			out = append(out, origin)
		}
	}
	return out
}
