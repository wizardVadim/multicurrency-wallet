package service_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/exchange/service"
)

type ratesProviderStub func(context.Context) (domain.ExchangeRates, error)

func (p ratesProviderStub) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	return p(ctx)
}

func TestGetExchangeRates(t *testing.T) {
	usd, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "EUR": 0.85, "RUB": 90})
	if err != nil {
		t.Fatal(err)
	}
	providerErr := errors.New("provider unavailable")
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "provider error"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			calls := 0
			svc := service.New(ratesProviderStub(func(got context.Context) (domain.ExchangeRates, error) {
				calls++
				if got != ctx {
					t.Error("provider did not receive caller context")
				}
				if fail {
					return domain.ExchangeRates{}, fmt.Errorf("get rates: %w", providerErr)
				}
				return rates, nil
			}), nil)
			got, err := svc.GetExchangeRates(ctx)
			if calls != 1 {
				t.Errorf("provider calls = %d; want 1", calls)
			}
			if fail {
				if !errors.Is(err, providerErr) {
					t.Fatalf("error = %v; want provider error", err)
				}
				if !reflect.DeepEqual(got, domain.ExchangeRates{}) {
					t.Error("unexpected rates on failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.BaseCurrency() != rates.BaseCurrency() || !reflect.DeepEqual(got.Rates(), rates.Rates()) {
				t.Error("returned rates differ from provider result")
			}
		})
	}
}

func TestGetExchangeRatesContextDone(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "canceled"
		if expired {
			name = "deadline exceeded"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if expired {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			} else {
				cancel()
			}
			defer cancel()
			svc := service.New(ratesProviderStub(func(context.Context) (domain.ExchangeRates, error) {
				t.Fatal("provider called with completed context")
				return domain.ExchangeRates{}, nil
			}), nil)
			got, err := svc.GetExchangeRates(ctx)
			if !errors.Is(err, ctx.Err()) {
				t.Errorf("error = %v; want %v", err, ctx.Err())
			}
			if !reflect.DeepEqual(got, domain.ExchangeRates{}) {
				t.Error("expected zero result")
			}
		})
	}
}
