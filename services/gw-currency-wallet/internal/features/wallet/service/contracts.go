package service

import (
	"context"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

type Repository interface {
	GetBalances(ctx context.Context, userID uuid.UUID) ([]domain.Balance, error)
	ApplyBalanceOperation(ctx context.Context, operation domain.BalanceOperation) error
}

type RatesProvider interface {
	GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error)
}
