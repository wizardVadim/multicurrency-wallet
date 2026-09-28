package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/wallet/service"

	"github.com/google/uuid"
)

type repositoryStub struct {
	balances       func(context.Context, uuid.UUID) ([]domain.Balance, error)
	t              *testing.T
	applyBalanceOp func(context.Context, domain.BalanceOperation) error
}

func (r *repositoryStub) ApplyBalanceOperation(ctx context.Context, operation domain.BalanceOperation) error {
	r.t.Helper()
	if r.applyBalanceOp == nil {
		r.t.Fatal("unexpected ApplyBalanceOperation call")
	}
	return r.applyBalanceOp(ctx, operation)
}

func (r *repositoryStub) GetBalances(ctx context.Context, id uuid.UUID) ([]domain.Balance, error) {
	r.t.Helper()
	if r.balances == nil {
		r.t.Fatal("unexpected GetBalances call")
	}
	return r.balances(ctx, id)
}

func TestGetBalances(t *testing.T) {
	id := uuid.New()
	currency, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := domain.NewBalance(id, currency, 12345)
	if err != nil {
		t.Fatal(err)
	}
	repoErr := errors.New("read balances failed")
	for _, tt := range []struct {
		name     string
		balances []domain.Balance
		err      error
		canceled bool
	}{
		{name: "success", balances: []domain.Balance{balance}},
		{name: "empty", balances: []domain.Balance{}},
		{name: "repository error", err: repoErr},
		{name: "canceled", err: context.Canceled, canceled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			calls := 0
			repo := &repositoryStub{t: t, balances: func(gotCtx context.Context, gotID uuid.UUID) ([]domain.Balance, error) {
				calls++
				if gotCtx != ctx || gotID != id {
					t.Error("incorrect context or user ID")
				}
				return tt.balances, tt.err
			}}
			got, err := service.New(repo, nil).GetBalances(ctx, id)
			if !errors.Is(err, tt.err) {
				t.Errorf("error = %v; want %v", err, tt.err)
			}
			if tt.err != nil {
				if len(got) != 0 {
					t.Error("error returned balances")
				}
			} else if !reflect.DeepEqual(got, tt.balances) {
				t.Error("incorrect balances")
			}
			wantCalls := 1
			if tt.canceled {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Errorf("calls = %d; want %d", calls, wantCalls)
			}
		})
	}
}

func TestApplyBalanceOperation(t *testing.T) {
	id := uuid.New()
	currency, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	op, err := domain.NewBalanceOperation(id, currency, domain.OperationTypeDeposit, 12345)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := domain.NewBalance(id, currency, 12345)
	if err != nil {
		t.Fatal(err)
	}
	readErr := errors.New("read after update failed")
	for _, tt := range []struct {
		name                       string
		applyErr, readErr, wantErr error
		canceled                   bool
	}{
		{name: "success"}, {name: "insufficient funds", applyErr: domain.ErrSmallBalance, wantErr: domain.ErrSmallBalance},
		{name: "overflow", applyErr: domain.ErrBalanceOverflow, wantErr: domain.ErrBalanceOverflow},
		{name: "missing balance", applyErr: domain.ErrBalanceNotFound, wantErr: domain.ErrBalanceNotFound},
		{name: "read failure after mutation", readErr: readErr, wantErr: readErr},
		{name: "canceled", canceled: true, wantErr: context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			applyCalls, readCalls := 0, 0
			repo := &repositoryStub{t: t, applyBalanceOp: func(c context.Context, got domain.BalanceOperation) error {
				applyCalls++
				if c != ctx || got != op {
					t.Error("incorrect mutation arguments")
				}
				return tt.applyErr
			}, balances: func(c context.Context, got uuid.UUID) ([]domain.Balance, error) {
				readCalls++
				if applyCalls != 1 || c != ctx || got != id {
					t.Error("incorrect read order or arguments")
				}
				if tt.readErr != nil {
					return nil, tt.readErr
				}
				return []domain.Balance{balance}, nil
			}}
			got, err := service.New(repo, nil).ApplyBalanceOperation(ctx, op)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error=%v; want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if !reflect.DeepEqual(got, []domain.Balance{balance}) {
					t.Error("incorrect returned balances")
				}
			} else if len(got) != 0 {
				t.Error("error returned balances")
			}
			wantApply, wantRead := 1, 1
			if tt.canceled {
				wantApply, wantRead = 0, 0
			} else if tt.applyErr != nil {
				wantRead = 0
			}
			if applyCalls != wantApply || readCalls != wantRead {
				t.Errorf("calls=%d/%d; want %d/%d", applyCalls, readCalls, wantApply, wantRead)
			}
		})
	}
}

type ratesProviderStub func(context.Context) (domain.ExchangeRates, error)

func (f ratesProviderStub) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	return f(ctx)
}

func TestBalanceOperationRatesPolicy(t *testing.T) {
	upstreamErr := errors.New("exchanger unavailable")
	usd, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	for _, operationType := range []domain.OperationType{domain.OperationTypeDeposit, domain.OperationTypeWithdraw} {
		for _, currencyType := range []domain.CurrencyType{domain.CurrencyTypeUSD, domain.CurrencyTypeEUR, domain.CurrencyTypeRUB} {
			for _, scenario := range []string{"success", "provider_error", "missing_rate", "canceled"} {
				t.Run(string(operationType)+"/"+string(currencyType)+"/"+scenario, func(t *testing.T) {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					if scenario == "canceled" {
						cancel()
					}
					currency, err := domain.NewCurrency(currencyType)
					if err != nil {
						t.Fatal(err)
					}
					op, err := domain.NewBalanceOperation(uuid.New(), currency, operationType, 100000)
					if err != nil {
						t.Fatal(err)
					}
					var calls []string
					provider := ratesProviderStub(func(c context.Context) (domain.ExchangeRates, error) {
						calls = append(calls, "rates")
						if c != ctx {
							t.Error("provider received different context")
						}
						if scenario == "provider_error" {
							return domain.ExchangeRates{}, upstreamErr
						}
						rates := domain.Rates{domain.CurrencyTypeUSD: 1}
						if scenario != "missing_rate" {
							rates[domain.CurrencyTypeEUR] = 0.87
							rates[domain.CurrencyTypeRUB] = 90.1
						}
						return domain.NewExchangeRates(usd, rates)
					})
					repo := &repositoryStub{t: t,
						applyBalanceOp: func(c context.Context, got domain.BalanceOperation) error {
							calls = append(calls, "apply")
							if c != ctx || got != op {
								t.Error("incorrect mutation arguments")
							}
							return nil
						},
						balances: func(c context.Context, id uuid.UUID) ([]domain.Balance, error) {
							calls = append(calls, "balances")
							if c != ctx || id != op.UserID() {
								t.Error("incorrect read arguments")
							}
							return nil, nil
						},
					}
					_, err = service.New(repo, provider).ApplyBalanceOperation(ctx, op)
					var wantErr error
					var wantCalls []string
					switch {
					case scenario == "canceled":
						wantErr = context.Canceled
					case currencyType == domain.CurrencyTypeUSD:
						wantCalls = []string{"apply", "balances"}
					case scenario == "provider_error":
						wantErr = upstreamErr
						wantCalls = []string{"rates"}
					case scenario == "missing_rate":
						wantErr = domain.ErrExchangeRateNotFound
						wantCalls = []string{"rates"}
					default:
						wantCalls = []string{"rates", "apply", "balances"}
					}
					if !errors.Is(err, wantErr) {
						t.Errorf("error = %v; want %v", err, wantErr)
					}
					if !reflect.DeepEqual(calls, wantCalls) {
						t.Errorf("calls = %v; want %v", calls, wantCalls)
					}
				})
			}
		}
	}
}
