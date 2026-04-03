package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "swagger": "2.0",
    "info": {
        "title": "24Alert Trading Bot API",
        "description": "REST API for the 24Alert trading bot gateway. Provides endpoints for orders, stop-orders, market data, portfolio, accounts, and risk management.",
        "version": "1.0.0",
        "contact": {
            "name": "24Alert Team"
        }
    },
    "host": "localhost:8080",
    "basePath": "/",
    "schemes": ["http"],
    "paths": {
        "/health": {
            "get": {
                "summary": "Health check",
                "tags": ["System"],
                "produces": ["application/json"],
                "responses": {
                    "200": { "description": "OK", "schema": { "type": "object", "properties": { "status": { "type": "string" } } } }
                }
            }
        },
        "/api/v1/orders": {
            "post": {
                "summary": "Place a new order",
                "tags": ["Orders"],
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "parameters": [
                    {
                        "in": "body",
                        "name": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "required": ["account_id", "instrument_uid", "quantity", "direction", "order_type"],
                            "properties": {
                                "account_id": { "type": "string" },
                                "instrument_uid": { "type": "string" },
                                "quantity": { "type": "integer" },
                                "direction": { "type": "string", "enum": ["buy", "sell"] },
                                "order_type": { "type": "string", "enum": ["limit", "market", "bestprice"] },
                                "price": { "type": "number" }
                            }
                        }
                    }
                ],
                "responses": {
                    "201": { "description": "Order placed" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            },
            "get": {
                "summary": "List active orders",
                "tags": ["Orders"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/orders/{id}": {
            "get": {
                "summary": "Get order state",
                "tags": ["Orders"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "id", "type": "string", "required": true },
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            },
            "put": {
                "summary": "Replace an order",
                "tags": ["Orders"],
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "id", "type": "string", "required": true },
                    { "in": "query", "name": "account_id", "type": "string", "required": true },
                    {
                        "in": "body",
                        "name": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "properties": {
                                "quantity": { "type": "integer" },
                                "price": { "type": "number" }
                            }
                        }
                    }
                ],
                "responses": {
                    "200": { "description": "Order replaced" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            },
            "delete": {
                "summary": "Cancel an order",
                "tags": ["Orders"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "id", "type": "string", "required": true },
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "Order cancelled" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/stop-orders": {
            "post": {
                "summary": "Place a new stop order",
                "tags": ["Stop Orders"],
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "parameters": [
                    {
                        "in": "body",
                        "name": "body",
                        "required": true,
                        "schema": {
                            "type": "object",
                            "required": ["account_id", "instrument_uid", "quantity", "direction", "stop_order_type", "stop_price"],
                            "properties": {
                                "account_id": { "type": "string" },
                                "instrument_uid": { "type": "string" },
                                "quantity": { "type": "integer" },
                                "direction": { "type": "string", "enum": ["buy", "sell"] },
                                "stop_order_type": { "type": "string", "enum": ["stop_loss", "take_profit", "stop_limit"] },
                                "stop_price": { "type": "number" },
                                "price": { "type": "number" }
                            }
                        }
                    }
                ],
                "responses": {
                    "201": { "description": "Stop order placed" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            },
            "get": {
                "summary": "List active stop orders",
                "tags": ["Stop Orders"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/stop-orders/{id}": {
            "delete": {
                "summary": "Cancel a stop order",
                "tags": ["Stop Orders"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "id", "type": "string", "required": true },
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "Stop order cancelled" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/candles": {
            "get": {
                "summary": "Get historical candles",
                "tags": ["Market Data"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "instrument_uid", "type": "string", "required": true },
                    { "in": "query", "name": "from", "type": "string", "format": "date-time" },
                    { "in": "query", "name": "to", "type": "string", "format": "date-time" },
                    { "in": "query", "name": "interval", "type": "string", "default": "1h", "enum": ["1m","5m","15m","1h","1d","1w","1M"] }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/orderbook/{uid}": {
            "get": {
                "summary": "Get order book",
                "tags": ["Market Data"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "uid", "type": "string", "required": true },
                    { "in": "query", "name": "depth", "type": "integer", "default": 20 }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/prices": {
            "get": {
                "summary": "Get last prices",
                "tags": ["Market Data"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "instrument_uid", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/trading-status/{uid}": {
            "get": {
                "summary": "Get instrument trading status",
                "tags": ["Market Data"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "uid", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/positions": {
            "get": {
                "summary": "Get account positions",
                "tags": ["Portfolio"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/portfolio": {
            "get": {
                "summary": "Get portfolio summary",
                "tags": ["Portfolio"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/limits": {
            "get": {
                "summary": "Get withdraw limits",
                "tags": ["Portfolio"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/operations": {
            "get": {
                "summary": "Get account operations",
                "tags": ["Portfolio"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "query", "name": "account_id", "type": "string", "required": true },
                    { "in": "query", "name": "instrument_uid", "type": "string" },
                    { "in": "query", "name": "from", "type": "string", "format": "date-time" },
                    { "in": "query", "name": "to", "type": "string", "format": "date-time" }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "400": { "description": "Bad request" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/accounts": {
            "get": {
                "summary": "List accounts",
                "tags": ["Accounts"],
                "produces": ["application/json"],
                "responses": {
                    "200": { "description": "OK" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/margin/{account_id}": {
            "get": {
                "summary": "Get margin attributes",
                "tags": ["Accounts"],
                "produces": ["application/json"],
                "parameters": [
                    { "in": "path", "name": "account_id", "type": "string", "required": true }
                ],
                "responses": {
                    "200": { "description": "OK" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/risk/status": {
            "get": {
                "summary": "Get risk service status",
                "tags": ["Risk"],
                "produces": ["application/json"],
                "responses": {
                    "200": { "description": "OK" },
                    "500": { "description": "Internal error" }
                }
            }
        },
        "/api/v1/risk/reset": {
            "post": {
                "summary": "Reset circuit breaker",
                "tags": ["Risk"],
                "produces": ["application/json"],
                "responses": {
                    "200": { "description": "Circuit breaker reset" },
                    "500": { "description": "Internal error" }
                }
            }
        }
    }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{"http"},
	Title:            "24Alert Trading Bot API",
	Description:      "REST API for the 24Alert trading bot gateway",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
