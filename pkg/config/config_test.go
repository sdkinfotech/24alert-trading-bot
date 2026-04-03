package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSandbox(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"false", "false", false},
		{"empty", "", false},
		{"random", "yes", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TINVEST_SANDBOX", tt.val)
			if got := IsSandbox(); got != tt.want {
				t.Errorf("IsSandbox() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTInvestToken_Sandbox(t *testing.T) {
	t.Setenv("TINVEST_SANDBOX", "true")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "sandbox-tok")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	tok, err := GetTInvestToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "sandbox-tok" {
		t.Errorf("got %q, want %q", tok, "sandbox-tok")
	}
}

func TestGetTInvestToken_Production(t *testing.T) {
	t.Setenv("TINVEST_SANDBOX", "false")
	t.Setenv("TINVEST_PROD_TOKEN", "prod-tok")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	tok, err := GetTInvestToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "prod-tok" {
		t.Errorf("got %q, want %q", tok, "prod-tok")
	}
}

func TestGetTInvestToken_Fallback(t *testing.T) {
	t.Setenv("TINVEST_SANDBOX", "false")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "fallback-tok")

	tok, err := GetTInvestToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "fallback-tok" {
		t.Errorf("got %q, want %q", tok, "fallback-tok")
	}
}

func TestGetTInvestToken_NoToken_Production(t *testing.T) {
	t.Setenv("TINVEST_SANDBOX", "false")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	_, err := GetTInvestToken()
	if err == nil {
		t.Fatal("expected error when no token set")
	}
	if !strings.Contains(err.Error(), "TINVEST_PROD_TOKEN") {
		t.Errorf("error should mention TINVEST_PROD_TOKEN: %v", err)
	}
}

func TestGetTInvestToken_NoToken_Sandbox(t *testing.T) {
	t.Setenv("TINVEST_SANDBOX", "true")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	_, err := GetTInvestToken()
	if err == nil {
		t.Fatal("expected error when no token set")
	}
	if !strings.Contains(err.Error(), "TINVEST_SANDBOX_TOKEN") {
		t.Errorf("error should mention TINVEST_SANDBOX_TOKEN: %v", err)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
tinvest:
  max_retries: 3
  call_timeout_sec: 10
services:
  gateway_port: 8080
risk:
  max_position_lots: 10
  circuit_breaker_threshold: 5
  circuit_breaker_cooldown: "2m"
  check_trading_session: true
logging:
  level: info
  format: json
  output: stdout
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TINVEST_SANDBOX", "true")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "test-token")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Risk.MaxPositionLots != 10 {
		t.Errorf("MaxPositionLots = %d, want 10", cfg.Risk.MaxPositionLots)
	}
	if cfg.Risk.CircuitBreakerThreshold != 5 {
		t.Errorf("CBThreshold = %d, want 5", cfg.Risk.CircuitBreakerThreshold)
	}
	if cfg.TInvest.Endpoint != "sandbox-invest-public-api.tbank.ru:443" {
		t.Errorf("Endpoint = %q, want sandbox endpoint", cfg.TInvest.Endpoint)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
tinvest:
  max_retries: 1
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TINVEST_SANDBOX", "false")
	t.Setenv("TINVEST_PROD_TOKEN", "")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	_, err := Load(cfgFile)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestLoad_ProductionEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
tinvest:
  max_retries: 1
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TINVEST_SANDBOX", "false")
	t.Setenv("TINVEST_PROD_TOKEN", "prod-tok")
	t.Setenv("TINVEST_SANDBOX_TOKEN", "")
	t.Setenv("TINVEST_TOKEN", "")

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.TInvest.Endpoint != "invest-public-api.tbank.ru:443" {
		t.Errorf("Endpoint = %q, want production endpoint", cfg.TInvest.Endpoint)
	}
}
