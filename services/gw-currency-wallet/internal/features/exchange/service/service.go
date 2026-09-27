package service

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

type Service struct {
	ratesProvider      RatesProvider
	exchangeRepository ExchangeRepository
}

func New(ratesProvider RatesProvider, exchangeRepository ExchangeRepository) *Service {
	return &Service{
		ratesProvider:      ratesProvider,
		exchangeRepository: exchangeRepository,
	}
}

func (s *Service) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExchangeRates{}, err
	}
	return s.ratesProvider.GetExchangeRates(ctx)
}

func (s *Service) Exchange(ctx context.Context, userID uuid.UUID, from, to domain.Currency, amount int64) (domain.ExchangeResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExchangeResult{}, err
	}

	if amount <= 0 {
		return domain.ExchangeResult{}, domain.ErrInvalidBalanceAmount
	}

	if from.IsEqual(to) {
		return domain.ExchangeResult{}, domain.ErrCurrenciesAreSame
	}

	exchangeRates, err := s.ratesProvider.GetExchangeRates(ctx)
	if err != nil {
		return domain.ExchangeResult{}, err
	}

	rates := exchangeRates.Rates()
	fromRate, ok := rates[from.CurrencyType()]
	if !ok {
		return domain.ExchangeResult{}, fmt.Errorf("from currency rate not found: %w", domain.ErrExchangeRateNotFound)
	}
	toRate, ok := rates[to.CurrencyType()]
	if !ok {
		return domain.ExchangeResult{}, fmt.Errorf("to currency rate not found: %w", domain.ErrExchangeRateNotFound)
	}

	exchangeAmount, err := calculateExchangeAmount(amount, fromRate, toRate)
	if err != nil {
		return domain.ExchangeResult{}, err
	}

	result, err := s.exchangeRepository.Exchange(ctx, userID, from, to, amount, exchangeAmount)
	if err != nil {
		return domain.ExchangeResult{}, err
	}

	return result, nil
}

func calculateExchangeAmount(amount int64, fromRate, toRate float32) (int64, error) {
	fromRateString := strconv.FormatFloat(float64(fromRate), 'f', -1, 32)
	fromRateBigRat, ok := new(big.Rat).SetString(fromRateString)
	if !ok {
		return 0, fmt.Errorf("%w: %g", errCouldntConvertIntoBigRat, fromRate)
	}

	toRateString := strconv.FormatFloat(float64(toRate), 'f', -1, 32)
	toRateBigRat, ok := new(big.Rat).SetString(toRateString)
	if !ok {
		return 0, fmt.Errorf("%w: %g", errCouldntConvertIntoBigRat, toRate)
	}

	amountBigRat := new(big.Rat).SetInt64(amount)

	result := new(big.Rat).Mul(amountBigRat, toRateBigRat)
	result.Quo(result, fromRateBigRat)

	// Amount and rates are positive: discard fractional minor units so
	// rounding cannot increase funds on a round trip at unchanged rates.
	whole := new(big.Int).Quo(result.Num(), result.Denom())

	if !whole.IsInt64() {
		return 0, domain.ErrBalanceOverflow
	}

	exchangedAmount := whole.Int64()
	if exchangedAmount <= 0 {
		return 0, domain.ErrInvalidBalanceAmount
	}

	return exchangedAmount, nil
}
