package config

import "testing"

func TestLoadAdminSettings(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "  owner@solvun.test  ")
	t.Setenv("ADMIN_PASSWORD_HASH", "  $2a$10$hash  ")
	t.Setenv("ADMIN_SESSION_TTL_MINUTES", "60")
	t.Setenv("ADMIN_COOKIE_SECURE", "false")

	cfg := Load()

	if cfg.AdminEmail != "owner@solvun.test" {
		t.Fatalf("AdminEmail = %q", cfg.AdminEmail)
	}
	if cfg.AdminPasswordHash != "$2a$10$hash" {
		t.Fatalf("AdminPasswordHash = %q", cfg.AdminPasswordHash)
	}
	if cfg.AdminSessionTTLMinutes != 60 {
		t.Fatalf("AdminSessionTTLMinutes = %d", cfg.AdminSessionTTLMinutes)
	}
	if cfg.AdminCookieSecure {
		t.Fatal("AdminCookieSecure should be false")
	}
}

func TestAdminDefaults(t *testing.T) {
	t.Setenv("ADMIN_SESSION_TTL_MINUTES", "")
	t.Setenv("ADMIN_COOKIE_SECURE", "")

	cfg := Load()

	if cfg.AdminSessionTTLMinutes != 720 {
		t.Fatalf("AdminSessionTTLMinutes = %d, want 720", cfg.AdminSessionTTLMinutes)
	}
	if !cfg.AdminCookieSecure {
		t.Fatal("AdminCookieSecure should default to true")
	}
	if cfg.AdminEmail != "" || cfg.AdminPasswordHash != "" {
		t.Fatal("admin credentials should default to empty")
	}
}

func TestOpsAlertSettings(t *testing.T) {
	t.Setenv("OPS_EMAIL", "  ops@solvun.test  ")
	t.Setenv("OPS_ALERT_INTERVAL_HOURS", "6")
	t.Setenv("OPS_ALERT_GRACE_MINUTES", "5")

	cfg := Load()

	if cfg.OpsEmail != "ops@solvun.test" {
		t.Fatalf("OpsEmail = %q", cfg.OpsEmail)
	}
	if cfg.OpsAlertIntervalHours != 6 {
		t.Fatalf("OpsAlertIntervalHours = %d", cfg.OpsAlertIntervalHours)
	}
	if cfg.OpsAlertGraceMinutes != 5 {
		t.Fatalf("OpsAlertGraceMinutes = %d", cfg.OpsAlertGraceMinutes)
	}
}

func TestOpsAlertDefaults(t *testing.T) {
	t.Setenv("OPS_EMAIL", "")
	t.Setenv("OPS_ALERT_INTERVAL_HOURS", "")
	t.Setenv("OPS_ALERT_GRACE_MINUTES", "")

	cfg := Load()

	if cfg.OpsEmail != "" {
		t.Fatalf("OpsEmail = %q, want empty (disabled)", cfg.OpsEmail)
	}
	if cfg.OpsAlertIntervalHours != 24 {
		t.Fatalf("OpsAlertIntervalHours = %d, want 24", cfg.OpsAlertIntervalHours)
	}
	if cfg.OpsAlertGraceMinutes != 10 {
		t.Fatalf("OpsAlertGraceMinutes = %d, want 10", cfg.OpsAlertGraceMinutes)
	}
}

func TestBiteshipWebhookToken(t *testing.T) {
	t.Setenv("BITESHIP_WEBHOOK_TOKEN", "  s3cret  ")

	if cfg := Load(); cfg.BiteshipWebhookToken != "s3cret" {
		t.Fatalf("BiteshipWebhookToken = %q", cfg.BiteshipWebhookToken)
	}
}

func TestShipmentRetrySettings(t *testing.T) {
	t.Setenv("SHIPMENT_RETRY_ENABLED", "false")
	t.Setenv("SHIPMENT_RETRY_MAX_ATTEMPTS", "3")
	t.Setenv("SHIPMENT_RETRY_INTERVAL_MINUTES", "20")

	cfg := Load()

	if cfg.ShipmentRetryEnabled {
		t.Fatal("ShipmentRetryEnabled should be false")
	}
	if cfg.ShipmentRetryMaxAttempts != 3 {
		t.Fatalf("ShipmentRetryMaxAttempts = %d, want 3", cfg.ShipmentRetryMaxAttempts)
	}
	if cfg.ShipmentRetryIntervalMinutes != 20 {
		t.Fatalf("ShipmentRetryIntervalMinutes = %d, want 20", cfg.ShipmentRetryIntervalMinutes)
	}

	t.Setenv("SHIPMENT_RETRY_ENABLED", "")
	t.Setenv("SHIPMENT_RETRY_MAX_ATTEMPTS", "")
	t.Setenv("SHIPMENT_RETRY_INTERVAL_MINUTES", "")

	cfg = Load()

	if !cfg.ShipmentRetryEnabled || cfg.ShipmentRetryMaxAttempts != 5 || cfg.ShipmentRetryIntervalMinutes != 10 {
		t.Fatalf("defaults = %v/%d/%d, want true/5/10",
			cfg.ShipmentRetryEnabled, cfg.ShipmentRetryMaxAttempts, cfg.ShipmentRetryIntervalMinutes)
	}
}

func TestRateLimitSetting(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "")

	if cfg := Load(); !cfg.RateLimitEnabled {
		t.Fatal("RateLimitEnabled should default to true")
	}

	t.Setenv("RATE_LIMIT_ENABLED", "false")

	if cfg := Load(); cfg.RateLimitEnabled {
		t.Fatal("RateLimitEnabled should be false when configured")
	}
}

func TestTrustProxy(t *testing.T) {
	t.Setenv("TRUST_PROXY", "")

	if cfg := Load(); cfg.TrustProxy {
		t.Fatal("TrustProxy should default to false")
	}

	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv("TRUST_PROXY", value)
		if cfg := Load(); !cfg.TrustProxy {
			t.Fatalf("TrustProxy should be true for %q", value)
		}
	}

	for _, value := range []string{"0", "false", "no", "off", "maybe"} {
		t.Setenv("TRUST_PROXY", value)
		if cfg := Load(); cfg.TrustProxy {
			t.Fatalf("TrustProxy should be false for %q", value)
		}
	}
}

func TestPaymentWindowGuard(t *testing.T) {
	cases := []struct {
		name   string
		expiry int
		ttl    int
		want   bool
	}{
		{"shorter window", 15, 20, false},
		{"equal", 20, 20, false},
		{"default 24h window", 0, 20, true},
		{"longer window", 30, 20, true},
		{"24h window with 24h ttl", 0, 24 * 60, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{MidtransExpiryMinutes: tc.expiry, OrderPendingTTLMinutes: tc.ttl}
			if got := cfg.PaymentWindowExceedsOrderTTL(); got != tc.want {
				t.Fatalf("PaymentWindowExceedsOrderTTL() = %v, want %v", got, tc.want)
			}
		})
	}
}
