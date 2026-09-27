package http

import (
	"context"
	"wallet-app/internal/core/domain"
)

type ExchangeService interface {
	GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error)
}
