package service_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/exchange/service"
)

type exchangeRepositoryStub func(context.Context, uuid.UUID, domain.Currency, domain.Currency, int64, int64) (domain.ExchangeResult, error)

func (f exchangeRepositoryStub) Exchange(ctx context.Context, id uuid.UUID, from, to domain.Currency, debit, credit int64) (domain.ExchangeResult, error) {
	return f(ctx, id, from, to, debit, credit)
}

func TestExchangeCalculation(t *testing.T) {
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	rub, _ := domain.NewCurrency(domain.CurrencyTypeRUB)
	for _, tt := range []struct {
		name     string
		from, to domain.Currency
		amount   int64
		eurRate  float32
		want     int64
		wantErr  error
	}{
		{"decimal rate", usd, rub, 1050, 0.875, 94605, nil},
		{"half rounds down", usd, eur, 100, 0.875, 87, nil},
		{"round down", usd, eur, 100, 0.874, 87, nil},
		{"above half rounds down", usd, eur, 100, 0.876, 87, nil},
		{"reverse", eur, usd, 875, 0.875, 1000, nil},
		{"cross rate", eur, rub, 875, 0.875, 90100, nil},
		{"fraction below one cent", usd, eur, 1, 0.99, 0, domain.ErrInvalidBalanceAmount},
		{"rounds to zero", rub, usd, 1, 0.875, 0, domain.ErrInvalidBalanceAmount},
		{"overflow", usd, rub, math.MaxInt64, 0.875, 0, domain.ErrBalanceOverflow},
		{"maximum exact", usd, eur, math.MaxInt64, 1, math.MaxInt64, nil},
		{"same currency", usd, usd, 100, 0.875, 0, domain.ErrCurrenciesAreSame},
		{"zero", usd, eur, 0, 0.875, 0, domain.ErrInvalidBalanceAmount},
		{"negative", usd, eur, -1, 0.875, 0, domain.ErrInvalidBalanceAmount},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "EUR": tt.eurRate, "RUB": 90.1})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			ctx := context.Background()
			id := uuid.New()
			repositoryCalls := 0
			repo := exchangeRepositoryStub(func(gotCtx context.Context, gotID uuid.UUID, from, to domain.Currency, debit, credit int64) (domain.ExchangeResult, error) {
				repositoryCalls++
				if gotCtx != ctx || gotID != id || from != tt.from || to != tt.to || debit != tt.amount || credit != tt.want {
					t.Errorf("unexpected repository arguments: %v %v %v %d %d", gotID, from, to, debit, credit)
				}
				return domain.NewExchangeResult(from, to, 123, 456, credit)
			})
			svc := service.New(ratesProviderStub(func(context.Context) (domain.ExchangeRates, error) { calls++; return rates, nil }), repo)
			got, err := svc.Exchange(ctx, id, tt.from, tt.to, tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != (domain.ExchangeResult{}) {
					t.Fatal("nonzero result on error")
				}
			} else if got.FromBalanceAmount() != 123 || got.ToBalanceAmount() != 456 || got.ExchangedAmount() != tt.want || got.FromCurrency() != tt.from || got.ToCurrency() != tt.to {
				t.Fatalf("result = %+v; want amount %d", got, tt.want)
			}
			wantRepositoryCalls := 0
			if tt.wantErr == nil && tt.from != tt.to {
				wantRepositoryCalls = 1
			}
			if repositoryCalls != wantRepositoryCalls {
				t.Errorf("repository calls = %d; want %d", repositoryCalls, wantRepositoryCalls)
			}
			wantCalls := 1
			if tt.amount <= 0 || tt.from == tt.to {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("provider calls = %d; want %d", calls, wantCalls)
			}
		})
	}
}

func TestExchangeMissingSourceRate(t *testing.T) {
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(ratesProviderStub(func(context.Context) (domain.ExchangeRates, error) { return rates, nil }), exchangeRepositoryStub(func(context.Context, uuid.UUID, domain.Currency, domain.Currency, int64, int64) (domain.ExchangeResult, error) {
		t.Fatal("repository called without source rate")
		return domain.ExchangeResult{}, nil
	}))
	defer func() {
		if value := recover(); value != nil {
			t.Errorf("missing source rate caused panic: %v", value)
		}
	}()
	got, err := svc.Exchange(context.Background(), uuid.New(), eur, usd, 100)
	if err == nil {
		t.Fatal("missing source rate must return an error")
	}
	if got != (domain.ExchangeResult{}) {
		t.Fatal("nonzero result on error")
	}
}

func TestExchangeRepositoryError(t *testing.T) {
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "EUR": 0.85})
	if err != nil {
		t.Fatal(err)
	}
	for _, wantErr := range []error{domain.ErrSmallBalance, domain.ErrBalanceOverflow, domain.ErrBalanceNotFound, errors.New("commit failed")} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			calls := 0
			svc := service.New(ratesProviderStub(func(context.Context) (domain.ExchangeRates, error) { return rates, nil }), exchangeRepositoryStub(func(context.Context, uuid.UUID, domain.Currency, domain.Currency, int64, int64) (domain.ExchangeResult, error) {
				calls++
				return domain.ExchangeResult{}, wantErr
			}))
			got, err := svc.Exchange(context.Background(), uuid.New(), usd, eur, 100)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v; want %v", err, wantErr)
			}
			if got != (domain.ExchangeResult{}) {
				t.Fatal("successful result returned after repository failure")
			}
			if calls != 1 {
				t.Fatalf("repository calls = %d; want 1", calls)
			}
		})
	}
}

func TestExchangeRoundTripDoesNotCreateMoney(t *testing.T) {
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	rub, _ := domain.NewCurrency(domain.CurrencyTypeRUB)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "RUB": 90.1, "EUR": 0.87})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	balances := map[domain.Currency]int64{rub: 1121382, eur: 0}
	repo := exchangeRepositoryStub(func(_ context.Context, gotID uuid.UUID, from, to domain.Currency, debit, credit int64) (domain.ExchangeResult, error) {
		if gotID != id {
			t.Fatal("wrong user")
		}
		if balances[from] < debit {
			t.Fatal("unexpected overdraft")
		}
		balances[from] -= debit
		balances[to] += credit
		return domain.NewExchangeResult(from, to, balances[from], balances[to], credit)
	})
	svc := service.New(ratesProviderStub(func(context.Context) (domain.ExchangeRates, error) { return rates, nil }), repo)
	forward, err := svc.Exchange(context.Background(), id, rub, eur, 1121382)
	if err != nil {
		t.Fatal(err)
	}
	if forward.ExchangedAmount() != 10827 {
		t.Fatalf("EUR cents = %d; want 10827", forward.ExchangedAmount())
	}
	backward, err := svc.Exchange(context.Background(), id, eur, rub, forward.ExchangedAmount())
	if err != nil {
		t.Fatal(err)
	}
	if backward.ExchangedAmount() != 1121278 {
		t.Fatalf("RUB kopecks = %d; want 1121278", backward.ExchangedAmount())
	}
	if balances[rub] > 1121382 || balances[eur] != 0 {
		t.Fatalf("unexpected final balances: %v", balances)
	}
}
