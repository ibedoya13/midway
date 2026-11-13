package ws

import (
	"os"
	"testing"
)

// TestConfigDefaultsAreSafeForLocalDevelopment checks that the defaults do
// not lock out a local developer.
func TestConfigDefaultsAreSafeForLocalDevelopment(t *testing.T) {
	cfg := ConfigFromEnv()

	if !cfg.AllowsOrigin("http://localhost:5173") {
		t.Fatal("the default config must allow the local dev server")
	}
	if !cfg.AllowsOrigin("http://192.168.0.2:5173") {
		t.Fatal("the default config must allow the local network")
	}
	if cfg.AllowsOrigin("https://attacker.example") {
		t.Fatal("the default config must not allow a foreign origin")
	}
}

// TestConfigAllowedOriginsFromEnvironment checks that the allowlist comes
// from the environment.
func TestConfigAllowedOriginsFromEnvironment(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://midway.amarrete.com,http://localhost:5173")

	cfg := ConfigFromEnv()

	if !cfg.AllowsOrigin("https://midway.amarrete.com") {
		t.Fatal("the configured origin must be allowed")
	}
	if !cfg.AllowsOrigin("http://localhost:5173") {
		t.Fatal("the configured origin must be allowed")
	}
	if cfg.AllowsOrigin("https://attacker.example") {
		t.Fatal("a foreign origin must be refused")
	}
}

// TestConfigAllowedOriginsWithSemicolons checks that the list can also be
// separated by semicolons: the gcloud CLI treats a comma as the separator of
// its own key=value arguments.
func TestConfigAllowedOriginsWithSemicolons(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://midway.amarrete.com;http://localhost:5173")

	cfg := ConfigFromEnv()

	if !cfg.AllowsOrigin("https://midway.amarrete.com") {
		t.Fatal("the configured origin must be allowed")
	}
	if !cfg.AllowsOrigin("http://localhost:5173") {
		t.Fatal("the configured origin must be allowed")
	}
	if cfg.AllowsOrigin("https://attacker.example") {
		t.Fatal("a foreign origin must be refused")
	}
}

// TestConfigIgnoresSurroundingSpaces checks that the list tolerates spaces.
func TestConfigIgnoresSurroundingSpaces(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", " https://midway.amarrete.com , http://localhost:5173 ")

	cfg := ConfigFromEnv()

	if !cfg.AllowsOrigin("https://midway.amarrete.com") {
		t.Fatal("the configured origin must be allowed")
	}
	if !cfg.AllowsOrigin("http://localhost:5173") {
		t.Fatal("the configured origin must be allowed")
	}
}

// TestConfigEmptyOriginIsDecidedExplicitly checks the rule for clients with
// no Origin header: non-browser callers such as probes and scripts.
func TestConfigEmptyOriginIsDecidedExplicitly(t *testing.T) {
	t.Setenv("ALLOW_EMPTY_ORIGIN", "true")
	cfg := ConfigFromEnv()
	if !cfg.AllowsOrigin("") {
		t.Fatal("an empty origin must be allowed when the flag is on")
	}

	t.Setenv("ALLOW_EMPTY_ORIGIN", "false")
	cfg = ConfigFromEnv()
	if cfg.AllowsOrigin("") {
		t.Fatal("an empty origin must be refused when the flag is off")
	}
}

// TestConfigEmptyOriginDefaultsToAllowed keeps the local probes working.
func TestConfigEmptyOriginDefaultsToAllowed(t *testing.T) {
	os.Unsetenv("ALLOW_EMPTY_ORIGIN")

	cfg := ConfigFromEnv()

	if !cfg.AllowsOrigin("") {
		t.Fatal("an empty origin must be allowed by default")
	}
}

// TestConfigRateLimitsHaveDefaults checks that the limits are set and
// tunable.
func TestConfigRateLimitsHaveDefaults(t *testing.T) {
	cfg := ConfigFromEnv()

	if cfg.MaxConnectionsPerIP <= 0 {
		t.Fatal("the connection limit must be positive")
	}
	if cfg.MessagesPerSecond <= 0 {
		t.Fatal("the message rate must be positive")
	}
	if cfg.MaxConnections <= 0 {
		t.Fatal("the total connection limit must be positive")
	}
}

// TestConfigRateLimitsFromEnvironment checks that the limits are read from
// the environment.
func TestConfigRateLimitsFromEnvironment(t *testing.T) {
	t.Setenv("MAX_CONNECTIONS_PER_IP", "3")
	t.Setenv("MESSAGES_PER_SECOND", "7")
	t.Setenv("MAX_CONNECTIONS", "11")

	cfg := ConfigFromEnv()

	if cfg.MaxConnectionsPerIP != 3 {
		t.Fatalf("MaxConnectionsPerIP = %d, want 3", cfg.MaxConnectionsPerIP)
	}
	if cfg.MessagesPerSecond != 7 {
		t.Fatalf("MessagesPerSecond = %d, want 7", cfg.MessagesPerSecond)
	}
	if cfg.MaxConnections != 11 {
		t.Fatalf("MaxConnections = %d, want 11", cfg.MaxConnections)
	}
}

// TestConfigInvalidValuesFallBackToTheDefaults checks that a broken value in
// the environment does not disable the limits.
func TestConfigInvalidValuesFallBackToTheDefaults(t *testing.T) {
	defaults := ConfigFromEnv()

	t.Setenv("MAX_CONNECTIONS_PER_IP", "not-a-number")
	t.Setenv("MESSAGES_PER_SECOND", "-5")

	cfg := ConfigFromEnv()

	if cfg.MaxConnectionsPerIP != defaults.MaxConnectionsPerIP {
		t.Fatalf("MaxConnectionsPerIP = %d, want the default %d",
			cfg.MaxConnectionsPerIP, defaults.MaxConnectionsPerIP)
	}
	if cfg.MessagesPerSecond != defaults.MessagesPerSecond {
		t.Fatalf("MessagesPerSecond = %d, want the default %d",
			cfg.MessagesPerSecond, defaults.MessagesPerSecond)
	}
}
