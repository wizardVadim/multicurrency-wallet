package cache_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/exchange/cache"
	"wallet-app/internal/features/exchange/service"
)

var _ service.RatesProvider = (*cache.Memory)(nil)

type providerFunc func(context.Context) (domain.ExchangeRates, error)

func (f providerFunc) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	return f(ctx)
}

func testRates(t *testing.T, eur float32) domain.ExchangeRates {
	t.Helper()
	usd, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "RUB": 90, "EUR": eur})
	if err != nil {
		t.Fatal(err)
	}
	return rates
}

func TestMemoryTTLAndRefreshFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first, second := testRates(t, 0.85), testRates(t, 0.9)
		upstreamErr := errors.New("upstream unavailable")
		calls := 0
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		m := cache.NewMemory(providerFunc(func(got context.Context) (domain.ExchangeRates, error) {
			calls++
			if got != ctx {
				t.Error("caller context not forwarded")
			}
			switch calls {
			case 1:
				return first, nil
			case 2:
				return domain.ExchangeRates{}, upstreamErr
			default:
				return second, nil
			}
		}), 30*time.Second)
		check := func(want domain.ExchangeRates, wantErr error, wantCalls int) {
			t.Helper()
			got, err := m.GetExchangeRates(ctx)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v; want %v", err, wantErr)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rates = %v; want %v", got, want)
			}
			if calls != wantCalls {
				t.Fatalf("provider calls = %d; want %d", calls, wantCalls)
			}
		}
		check(first, nil, 1)
		time.Sleep(29 * time.Second)
		check(first, nil, 1)
		time.Sleep(time.Second)
		check(domain.ExchangeRates{}, upstreamErr, 2)
		// A failed refresh must not extend TTL or cache the error.
		check(second, nil, 3)
		check(second, nil, 3)
	})
}

func TestMemoryConcurrentMiss(t *testing.T) {
	rates := testRates(t, 0.85)
	var calls atomic.Int32
	m := cache.NewMemory(providerFunc(func(context.Context) (domain.ExchangeRates, error) {
		calls.Add(1)
		return rates, nil
	}), time.Hour)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := m.GetExchangeRates(context.Background())
			if err != nil || !reflect.DeepEqual(got, rates) {
				t.Errorf("rates = %v, error = %v", got, err)
			}
			copy := got.Rates()
			copy[domain.CurrencyTypeEUR] = 0
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d; want 1", calls.Load())
	}
}

func TestMemoryCanceledContext(t *testing.T) {
	for _, warm := range []bool{false, true} {
		name := "empty"
		if warm {
			name = "fresh"
		}
		t.Run(name, func(t *testing.T) {
			rates := testRates(t, 0.85)
			calls := 0
			m := cache.NewMemory(providerFunc(func(context.Context) (domain.ExchangeRates, error) {
				calls++
				return rates, nil
			}), time.Hour)
			if warm {
				if _, err := m.GetExchangeRates(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			before := calls
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			got, err := m.GetExchangeRates(ctx)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, domain.ExchangeRates{}) {
				t.Fatalf("rates = %v, error = %v; want empty result and cancellation", got, err)
			}
			if calls != before {
				t.Error("provider called with canceled context")
			}
		})
	}
}
