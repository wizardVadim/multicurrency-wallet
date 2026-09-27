package exchange_http

import (
	"context"

	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

type ExchangeService interface {
	GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error)
	Exchange(ctx context.Context, userID uuid.UUID, from, to domain.Currency, amount int64) (domain.ExchangeResult, error)
}
