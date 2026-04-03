package tinvest

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// Client wraps the T-Invest Go SDK client
type Client struct {
	underlying *investgo.Client
	logger     *slog.Logger
	config     investgo.Config
}

// SimpleLogger adapter for SDK (implements investgo.Logger interface)
type SimpleLogger struct {
	*log.Logger
}

// NewSimpleLogger creates a logger adapter for SDK
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{
		Logger: log.New(os.Stdout, "[T-Invest] ", log.LstdFlags),
	}
}

// Errorf implements investgo.Logger interface
func (sl *SimpleLogger) Errorf(format string, v ...interface{}) {
	sl.Printf("ERROR: "+format, v...)
}

// Infof implements investgo.Logger interface
func (sl *SimpleLogger) Infof(format string, v ...interface{}) {
	sl.Printf("INFO: "+format, v...)
}

// Debugf implements investgo.Logger interface
func (sl *SimpleLogger) Debugf(format string, v ...interface{}) {
	sl.Printf("DEBUG: "+format, v...)
}

// NewTInvestClient creates a new T-Invest client
func NewTInvestClient(ctx context.Context, endpoint, token string, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if token == "" {
		return nil, fmt.Errorf("T-Invest token is empty")
	}

	config := investgo.Config{
		EndPoint:                     endpoint,
		Token:                        token,
		AppName:                      "24alert-trading-bot",
		MaxRetries:                   3,
		DisableResourceExhaustedRetry: false,
	}

	sdkLogger := NewSimpleLogger()
	client, err := investgo.NewClient(ctx, config, sdkLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create T-Invest client: %w", err)
	}

	return &Client{
		underlying: client,
		logger:     logger,
		config:     config,
	}, nil
}

// Stop gracefully closes the client connection
func (c *Client) Stop() {
	if c.underlying != nil {
		c.underlying.Stop()
	}
}

// OrdersServiceClient returns the orders service client
func (c *Client) OrdersServiceClient() *investgo.OrdersServiceClient {
	return c.underlying.NewOrdersServiceClient()
}

// MarketDataServiceClient returns the market data service client
func (c *Client) MarketDataServiceClient() *investgo.MarketDataServiceClient {
	return c.underlying.NewMarketDataServiceClient()
}

// MarketDataStreamClient returns the market data stream client
func (c *Client) MarketDataStreamClient() *investgo.MarketDataStreamClient {
	return c.underlying.NewMarketDataStreamClient()
}

// OperationsServiceClient returns the operations (portfolio) service client
func (c *Client) OperationsServiceClient() *investgo.OperationsServiceClient {
	return c.underlying.NewOperationsServiceClient()
}

// UsersServiceClient returns the users service client
func (c *Client) UsersServiceClient() *investgo.UsersServiceClient {
	return c.underlying.NewUsersServiceClient()
}

// InstrumentsServiceClient returns the instruments service client
func (c *Client) InstrumentsServiceClient() *investgo.InstrumentsServiceClient {
	return c.underlying.NewInstrumentsServiceClient()
}

// StopOrdersServiceClient returns the stop orders service client
func (c *Client) StopOrdersServiceClient() *investgo.StopOrdersServiceClient {
	return c.underlying.NewStopOrdersServiceClient()
}

// OrdersStreamClient returns the orders stream client for subscribing to order/trade streams
func (c *Client) OrdersStreamClient() *investgo.OrdersStreamClient {
	return c.underlying.NewOrdersStreamClient()
}

// OperationsStreamClient returns the operations stream client for portfolio/positions streaming
func (c *Client) OperationsStreamClient() *investgo.OperationsStreamClient {
	return c.underlying.NewOperationsStreamClient()
}

// SandboxServiceClient returns the sandbox service client (for testing)
func (c *Client) SandboxServiceClient() *investgo.SandboxServiceClient {
	return c.underlying.NewSandboxServiceClient()
}

// IsProductionEndpoint checks if client is configured for production
func (c *Client) IsProductionEndpoint() bool {
	return c.config.EndPoint == "invest-public-api.tbank.ru:443"
}

// IsSandboxEndpoint checks if client is configured for sandbox
func (c *Client) IsSandboxEndpoint() bool {
	return c.config.EndPoint == "sandbox-invest-public-api.tbank.ru:443"
}
