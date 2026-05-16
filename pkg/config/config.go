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
	Strategies       StrategiesRunnerConfig `mapstructure:"strategies"`
	Redis            RedisConfig            `mapstructure:"redis"`
	Features         FeaturesConfig         `mapstructure:"features"`
}

// RedisConfig contains Redis connection settings (used for candle cache).
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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

// StrategiesRunnerConfig configures the standalone strategy-runner process.
type StrategiesRunnerConfig struct {
	RunnerPort          int                       `mapstructure:"runner_port"`
	MetricsPort         int                       `mapstructure:"metrics_port"`
	EvaluationTimeoutMS int                       `mapstructure:"evaluation_timeout_ms"`
	JournalPath         string                    `mapstructure:"journal_path"`
	TradingSchedule     TradingScheduleConfig     `mapstructure:"trading_schedule"`
	Watchdog            WatchdogRunnerConfig      `mapstructure:"watchdog"`
	Notifications       NotificationsRunnerConfig `mapstructure:"notifications"`
	Instances           []StrategyInstanceConfig  `mapstructure:"instances"`
}

// TradingScheduleConfig defines when the runner is allowed to submit orders.
// Outside this window signals are blocked. Format: "HH:MM".
type TradingScheduleConfig struct {
	MainStart string `mapstructure:"main_start"` // default "10:00"
	MainEnd   string `mapstructure:"main_end"`   // default "18:39"
	Timezone  string `mapstructure:"timezone"`   // default "Europe/Moscow"
}

// WatchdogRunnerConfig configures periodic self-checks in strategy-runner.
type WatchdogRunnerConfig struct {
	Enabled            bool    `mapstructure:"enabled"`
	CheckIntervalSec   int     `mapstructure:"check_interval_sec"`
	MaxDrawdownPercent float64 `mapstructure:"max_drawdown_percent"`
	MaxDailyLossRub    float64 `mapstructure:"max_daily_loss_rub"`
	PauseOnDrawdown    bool    `mapstructure:"pause_on_drawdown"`
	StuckOrderMinutes  int     `mapstructure:"stuck_order_minutes"`
}

// NotificationsRunnerConfig configures outbound notifications from strategy-runner.
type NotificationsRunnerConfig struct {
	Telegram TelegramNotifyConfig `mapstructure:"telegram"`
}

// TelegramNotifyConfig Telegram bot notifications.
type TelegramNotifyConfig struct {
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
}

// StrategyInstanceConfig is one running strategy bound to an account and instruments.
type StrategyInstanceConfig struct {
	ID          string            `mapstructure:"id"`
	Type        string            `mapstructure:"type"`
	AccountID   string            `mapstructure:"account_id"`
	Instruments []string          `mapstructure:"instruments"`
	Enabled     bool              `mapstructure:"enabled"`
	Params      map[string]string `mapstructure:"params"`
	Endpoint    string            `mapstructure:"endpoint"` // for type "grpc"
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
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")

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

// LoadStrategiesOnly reads config.yaml and returns only the strategies section,
// without validating the T-Invest token. Used for hot-reload at runtime.
func LoadStrategiesOnly(configPath string) (*StrategiesRunnerConfig, error) {
	v := viper.New()
	if configPath == "" {
		configPath = "config/config.yaml"
	}
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg.Strategies, nil
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
