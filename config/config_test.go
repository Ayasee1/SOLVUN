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
