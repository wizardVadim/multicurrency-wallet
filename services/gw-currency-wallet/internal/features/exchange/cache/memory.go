package cache

import (
	"context"
	"sync"
	"time"
	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/exchange/service"
)

type Memory struct {
	provider  service.RatesProvider
	ttl       time.Duration
	mu        sync.Mutex
	rates     domain.ExchangeRates
	expiresAt time.Time
}

func NewMemory(provider service.RatesProvider, ttl time.Duration) *Memory {
	return &Memory{
		provider: provider,
		ttl:      ttl,
	}
}

func (cache *Memory) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.ExchangeRates{}, err
	}
	isActual := time.Now().Before(cache.expiresAt)
	if !isActual {
		rates, err := cache.provider.GetExchangeRates(ctx)
		if err != nil {
			return domain.ExchangeRates{}, err
		}
		cache.rates = rates
		cache.expiresAt = time.Now().Add(cache.ttl)
	}
	return cache.rates, nil
}
