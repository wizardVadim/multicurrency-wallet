package domain_test

import (
	"errors"
	"math"
	"testing"
	"wallet-app/internal/core/domain"
)

func TestExchangeResult(t *testing.T) {
	currencies := make([]domain.Currency, 0, 3)
	for _, code := range []domain.CurrencyType{domain.CurrencyTypeUSD, domain.CurrencyTypeEUR, domain.CurrencyTypeRUB} {
		currency, err := domain.NewCurrency(code)
		if err != nil {
			t.Fatal(err)
		}
		currencies = append(currencies, currency)
	}
	for _, from := range currencies {
		for _, to := range currencies {
			t.Run(string(from.CurrencyType())+"_"+string(to.CurrencyType()), func(t *testing.T) {
				for _, amounts := range [][3]int64{
					{123, 456, 78},
					{0, 0, 0},
					{0, 100, 1},
					{100, 0, 1},
					{100, 200, 0},
					{math.MaxInt64, 200, 1},
					{100, math.MaxInt64, 1},
					{100, 200, math.MaxInt64},
				} {
					got, err := domain.NewExchangeResult(from, to, amounts[0], amounts[1], amounts[2])
					if err != nil {
						t.Fatalf("amounts %v: %v", amounts, err)
					}
					if got.FromCurrency() != from || got.ToCurrency() != to || got.FromBalanceAmount() != amounts[0] || got.ToBalanceAmount() != amounts[1] || got.ExchangedAmount() != amounts[2] {
						t.Fatalf("result does not preserve currencies, balances and exchanged amount: %+v", got)
					}
				}
			})
		}
	}
	usd, eur := currencies[0], currencies[1]
	for _, tt := range []struct {
		name                         string
		from, to                     domain.Currency
		fromAmount, toAmount, amount int64
		wantErr                      error
	}{
		{"invalid source currency", domain.Currency{}, eur, 100, 200, 50, domain.ErrInvalidCurrencyType},
		{"invalid target currency", usd, domain.Currency{}, 100, 200, 50, domain.ErrInvalidCurrencyType},
		{"both currencies invalid", domain.Currency{}, domain.Currency{}, 100, 200, 50, domain.ErrInvalidCurrencyType},
		{"negative source balance", usd, eur, -1, 200, 50, domain.ErrInvalidBalanceAmount},
		{"minimum source balance", usd, eur, math.MinInt64, 200, 50, domain.ErrInvalidBalanceAmount},
		{"negative target balance", usd, eur, 100, -1, 50, domain.ErrInvalidBalanceAmount},
		{"minimum target balance", usd, eur, 100, math.MinInt64, 50, domain.ErrInvalidBalanceAmount},
		{"negative exchanged amount", usd, eur, 100, 200, -1, domain.ErrInvalidBalanceAmount},
		{"minimum exchanged amount", usd, eur, 100, 200, math.MinInt64, domain.ErrInvalidBalanceAmount},
		{"same currency negative amount", usd, usd, 100, 100, -1, domain.ErrInvalidBalanceAmount},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewExchangeResult(tt.from, tt.to, tt.fromAmount, tt.toAmount, tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
			if got != (domain.ExchangeResult{}) {
				t.Fatal("invalid result must be zero")
			}
		})
	}
}
