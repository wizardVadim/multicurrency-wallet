package service

import (
	"context"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

type RatesProvider interface {
	GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error)
}

type ExchangeRepository interface {
	Exchange(ctx context.Context, userID uuid.UUID, from, to domain.Currency, fromAmount, toAmount int64) (domain.ExchangeResult, error)
}
