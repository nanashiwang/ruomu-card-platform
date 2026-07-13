# 若木云卡 API

若木云卡 API 提供公开端、用户鉴权、订单支付、人工交付和管理后台接口。

## Tech Stack

- Go
- Gin
- GORM
- SQLite / PostgreSQL

## What This Service Does

- Serves REST APIs for user, order, and payment flows
- Handles payment callbacks/webhooks
- Supports product, fulfillment, and configuration management

## Quick Start

```bash
go mod tidy
go run cmd/server/main.go
```

The default health check endpoint is:

- `GET /health`

## Online Documentation

- https://rm.meta-api.vip/api/v1/public/config
