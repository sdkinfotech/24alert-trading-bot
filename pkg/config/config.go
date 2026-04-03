package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	TInvest          TInvestConfig          `mapstructure:"tinvest"`
	Services         ServicesConfig         `mapstructure:"services"`
	Risk             RiskConfig             `mapstructure:"risk"`
	Logging          LoggingConfig          `mapstructure:"logging"`
	Metrics          MetricsConfig          `mapstructure:"metrics"`
	RateLimits       map[string]int         `mapstructure:"rate_limits"`
	MarketDataStream MarketDataStreamConfig `mapstructure:"market_data_stream"`
	Strategy         StrategyConfig         `mapstructure:"strategy"`
	Features         FeaturesConfig         `mapstructure:"features"`
}

// TInvestConfig contains T-Invest API settings
type TInvestConfig struct {
	Endpoint                      string `mapstructure:"endpoint"`
	MaxRetries                    int    `mapstructure:"max_retries"`
	DisableResourceExhaustedRetry bool   `mapstructure:"disable_resource_exhausted_retry"`
	CallTimeoutSec                int    `mapstructure:"call_timeout_sec"`
	SandboxAccountID              string `mapstructure:"sandbox_account_id"`
}

// ServicesConfig contains service port configuration
type ServicesConfig struct {
	OrderPort      int `mapstructure:"order_port"`
	MarketDataPort int `mapstructure:"marketdata_port"`
	PortfolioPort  int `mapstructure:"portfolio_port"`
	RiskPort       int `mapstructure:"risk_port"`
	GatewayPort    int `mapstructure:"gateway_port"`
}

// RiskConfig contains risk management settings
type RiskConfig struct {
	MaxPositionLots            int           `mapstructure:"max_position_lots"`
	CircuitBreakerThreshold    int           `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerCooldownStr  string        `mapstructure:"circuit_breaker_cooldown"`
	CircuitBreakerCooldown     time.Duration `mapstructure:"-"` // parsed from string
	MarginCallThresholdPercent int           `mapstructure:"margin_call_threshold_percent"`
	CheckTradingSession        bool          `mapstructure:"check_trading_session"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level                string `mapstructure:"level"`
	Format               string `mapstructure:"format"`
	Output               string `mapstructure:"output"`
	FilePath             string `mapstructure:"file_path"`
	IncludeCorrelationID bool   `mapstructure:"include_correlation_id"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Endpoint           string `mapstructure:"endpoint"`
	DetailedHistograms bool   `mapstructure:"detailed_histograms"`
}

// MarketDataStreamConfig contains streaming settings
type MarketDataStreamConfig struct {
	MaxSubscriptions     int `mapstructure:"max_subscriptions"`
	MaxConcurrentStreams int `mapstructure:"max_concurrent_streams"`
	ReconnectDelayMs     int `mapstructure:"reconnect_delay_ms"`
	MaxReconnectAttempts int `mapstructure:"max_reconnect_attempts"`
}

// StrategyConfig contains strategy plugin settings
type StrategyConfig struct {
	Endpoint            string                 `mapstructure:"endpoint"`
	EvaluationTimeoutMs int                    `mapstructure:"evaluation_timeout_ms"`
	Config              map[string]interface{} `mapstructure:"config"`
}

// FeaturesConfig contains feature flags
type FeaturesConfig struct {
	EnableRiskValidation bool `mapstructure:"enable_risk_validation"`
	EnableOrderJournal   bool `mapstructure:"enable_order_journal"`
	EnableMDCache        bool `mapstructure:"enable_md_cache"`
}

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath == "" {
		configPath = "config/config.yaml"
	}

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	_ = v.BindEnv("tinvest.endpoint", "TINVEST_ENDPOINT")
	_ = v.BindEnv("tinvest.sandbox_account_id", "TINVEST_SANDBOX_ACCOUNT_ID")
	_ = v.BindEnv("logging.level", "LOG_LEVEL")

	v.AutomaticEnv()
	v.SetEnvPrefix("24ALERT")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	if cfg.Risk.CircuitBreakerCooldownStr != "" {
		d, err := time.ParseDuration(cfg.Risk.CircuitBreakerCooldownStr)
		if err != nil {
			return nil, fmt.Errorf("invalid circuit_breaker_cooldown: %w", err)
		}
		cfg.Risk.CircuitBreakerCooldown = d
	} else {
		cfg.Risk.CircuitBreakerCooldown = 5 * time.Minute
	}

	if _, err := GetTInvestToken(); err != nil {
		return nil, err
	}

	if IsSandbox() {
		cfg.TInvest.Endpoint = "sandbox-invest-public-api.tbank.ru:443"
	} else {
		cfg.TInvest.Endpoint = "invest-public-api.tbank.ru:443"
	}

	return cfg, nil
}

// IsSandbox returns true when TINVEST_SANDBOX env var is "true" or "1".
func IsSandbox() bool {
	v := os.Getenv("TINVEST_SANDBOX")
	return v == "true" || v == "1"
}

// GetTInvestToken resolves the active API token based on the current mode:
//   - sandbox: TINVEST_SANDBOX_TOKEN → fallback TINVEST_TOKEN
//   - production: TINVEST_PROD_TOKEN → fallback TINVEST_TOKEN
func GetTInvestToken() (string, error) {
	if IsSandbox() {
		if t := os.Getenv("TINVEST_SANDBOX_TOKEN"); t != "" {
			return t, nil
		}
	} else {
		if t := os.Getenv("TINVEST_PROD_TOKEN"); t != "" {
			return t, nil
		}
	}
	if t := os.Getenv("TINVEST_TOKEN"); t != "" {
		return t, nil
	}
	if IsSandbox() {
		return "", fmt.Errorf("TINVEST_SANDBOX_TOKEN (or TINVEST_TOKEN) environment variable not set")
	}
	return "", fmt.Errorf("TINVEST_PROD_TOKEN (or TINVEST_TOKEN) environment variable not set")
}
