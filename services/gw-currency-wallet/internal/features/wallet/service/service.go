package service

import (
	"context"
	"contracts/events"
	"strconv"
	"time"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

type Service struct {
	repo          Repository
	ratesProvider RatesProvider
}

func New(repository Repository, ratesProvider RatesProvider) *Service {
	return &Service{
		repo:          repository,
		ratesProvider: ratesProvider,
	}
}

func (s *Service) GetBalances(ctx context.Context, userID uuid.UUID) ([]domain.Balance, error) {
	if err := ctx.Err(); err != nil {
		return []domain.Balance{}, err
	}
	return s.repo.GetBalances(ctx, userID)
}

func (s *Service) ApplyBalanceOperation(ctx context.Context, operation domain.BalanceOperation) ([]domain.Balance, error) {
	if err := ctx.Err(); err != nil {
		return []domain.Balance{}, err
	}

	currentRate, err := s.operationRate(ctx, operation.Currency())
	if err != nil {
		return nil, err
	}

	_, err = prepareLargeOperation(operation, currentRate)
	if err != nil {
		return []domain.Balance{}, err
	}

	if err := s.repo.ApplyBalanceOperation(ctx, operation); err != nil {
		return []domain.Balance{}, err
	}

	return s.repo.GetBalances(ctx, operation.UserID())
}

func (s *Service) operationRate(ctx context.Context, currency domain.Currency) (string, error) {
	var rate string
	if currency.CurrencyType() != domain.CurrencyTypeUSD {
		exchangeRates, err := s.ratesProvider.GetExchangeRates(ctx)
		if err != nil {
			return "", err
		}
		floatRate, ok := exchangeRates.Rates()[currency.CurrencyType()]
		if !ok {
			return "", domain.ErrExchangeRateNotFound
		}
		rate = strconv.FormatFloat(float64(floatRate), 'f', -1, 32)
	} else {
		rate = "1"
	}
	return rate, nil
}

func prepareLargeOperation(operation domain.BalanceOperation, rate string) (*events.LargeOperation, error) {
	res, err := domain.IsLargeOperation(operation.Amount(), rate)
	if err != nil {
		return nil, err
	}
	if !res {
		return nil, nil
	}
	largeOperation := &events.LargeOperation{
		SchemaVersion: 1,
		OperationType: string(operation.OperationType()),
		EventID:       uuid.NewString(),
		TransactionID: uuid.NewString(),
		Status:        "succeeded",
		Currency:      string(operation.Currency().CurrencyType()),
		OccurredAt:    time.Now().UTC(),
		UserID:        operation.UserID().String(),
		AmountMinor:   operation.Amount(),
		UnitsPerUSD:   rate,
	}

	if err := largeOperation.Validate(); err != nil {
		return nil, err
	}
	return largeOperation, nil
}
