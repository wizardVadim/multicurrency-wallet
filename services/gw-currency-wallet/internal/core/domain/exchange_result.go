package domain

import "fmt"

type ExchangeResult struct {
	fromCurrency      Currency
	toCurrency        Currency
	fromBalanceAmount int64
	toBalanceAmount   int64
	exchangedAmount   int64
}

func NewExchangeResult(from, to Currency, fromAmount, toAmount, amount int64) (ExchangeResult, error) {
	result := ExchangeResult{
		fromCurrency:      from,
		toCurrency:        to,
		fromBalanceAmount: fromAmount,
		toBalanceAmount:   toAmount,
		exchangedAmount:   amount,
	}

	if err := result.validate(); err != nil {
		return ExchangeResult{}, err
	}

	return result, nil
}

func (result ExchangeResult) validate() error {
	if err := result.fromCurrency.validate(); err != nil {
		return fmt.Errorf("from currency: %w", err)
	}
	if err := result.toCurrency.validate(); err != nil {
		return fmt.Errorf("to currency: %w", err)
	}
	if result.fromBalanceAmount < 0 {
		return fmt.Errorf("from balance amount: %w", ErrInvalidBalanceAmount)
	}
	if result.toBalanceAmount < 0 {
		return fmt.Errorf("to balance amount: %w", ErrInvalidBalanceAmount)
	}
	if result.exchangedAmount < 0 {
		return fmt.Errorf("exchanged balance amount: %w", ErrInvalidBalanceAmount)
	}

	return nil
}

func (result ExchangeResult) FromCurrency() Currency {
	return result.fromCurrency
}

func (result ExchangeResult) ToCurrency() Currency {
	return result.toCurrency
}

func (result ExchangeResult) ExchangedAmount() int64 {
	return result.exchangedAmount
}

func (result ExchangeResult) FromBalanceAmount() int64 {
	return result.fromBalanceAmount
}

func (result ExchangeResult) ToBalanceAmount() int64 {
	return result.toBalanceAmount
}
