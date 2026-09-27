package repository_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"os"
	"testing"
	"time"
	"wallet-app/internal/core/domain"
	"wallet-app/internal/features/exchange/repository"
	"wallet-app/internal/features/exchange/service"
)

var _ service.ExchangeRepository = (*repository.PostgresRepository)(nil)

func setupExchangeDatabase(t *testing.T) (context.Context, *repository.PostgresRepository, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(admin.Close)
	schema := pgx.Identifier{"exchange_test_" + uuid.New().String()}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}

	cfg.MaxConns = 8
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)
	migration, err := os.ReadFile("../../../../migrations/000001_create_wallets.up.sql")

	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	for _, name := range []string{"000002_create_users.up.sql", "000003_multicurrency_wallets.up.sql"} {
		sql, err := os.ReadFile("../../../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}

	return ctx, repository.NewPostgresRepository(pool), pool
}

func seed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, usd, eur int64) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users(id,username,email,password_hash) VALUES ($1,$2,$3,'hash')", id, id.String(), id.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO balances(user_id,currency,amount) VALUES ($1,'USD',$2),($1,'EUR',$3),($1,'RUB',77)", id, usd, eur); err != nil {
		t.Fatal(err)
	}
	return id
}
func assertBalances(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, usd, eur int64) {
	t.Helper()
	for code, want := range map[string]int64{"USD": usd, "EUR": eur, "RUB": 77} {
		var got int64
		if err := pool.QueryRow(ctx, "SELECT amount FROM balances WHERE user_id=$1 AND currency=$2", id, code).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s balance = %d; want %d", code, got, want)
		}
	}
}
func TestExchangePostgres(t *testing.T) {
	for _, tt := range []struct {
		name                                      string
		reverse                                   bool
		usd, eur, debit, credit, wantUSD, wantEUR int64
		wantErr                                   error
	}{
		{"USD to EUR", false, 1000, 200, 300, 250, 700, 450, nil},
		{"EUR to USD", true, 1000, 200, 100, 120, 1120, 100, nil},
		{"entire balance", false, 1000, 0, 1000, 850, 0, 850, nil},
		{"insufficient", false, 100, 200, 101, 90, 100, 200, domain.ErrSmallBalance},
		{"overflow", false, 100, math.MaxInt64, 1, 1, 100, math.MaxInt64, domain.ErrBalanceOverflow},
		{"maximum allowed", false, 100, math.MaxInt64 - 1, 1, 1, 99, math.MaxInt64, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, repo, pool := setupExchangeDatabase(t)
			id := seed(t, ctx, pool, tt.usd, tt.eur)
			other := seed(t, ctx, pool, 999, 888)
			from, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
			to, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
			if tt.reverse {
				from, to = to, from
			}
			got, err := repo.Exchange(ctx, id, from, to, tt.debit, tt.credit)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
			if err == nil {
				wantFrom, wantTo := tt.wantUSD, tt.wantEUR
				if tt.reverse {
					wantFrom, wantTo = wantTo, wantFrom
				}
				if got.FromCurrency() != from || got.ToCurrency() != to || got.FromBalanceAmount() != wantFrom || got.ToBalanceAmount() != wantTo || got.ExchangedAmount() != tt.credit {
					t.Errorf("result = %+v; want balances %d/%d, exchanged amount %d", got, wantFrom, wantTo, tt.credit)
				}
			} else if got != (domain.ExchangeResult{}) {
				t.Error("nonzero result on error")
			}
			assertBalances(t, ctx, pool, id, tt.wantUSD, tt.wantEUR)
			assertBalances(t, ctx, pool, other, 999, 888)
		})
	}
}
func TestExchangeMissingBalance(t *testing.T) {
	for _, missing := range []string{"USD", "EUR"} {
		t.Run(missing, func(t *testing.T) {
			ctx, repo, pool := setupExchangeDatabase(t)
			id := seed(t, ctx, pool, 100, 200)
			if _, err := pool.Exec(ctx, "DELETE FROM balances WHERE user_id=$1 AND currency=$2", id, missing); err != nil {
				t.Fatal(err)
			}
			usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
			eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
			if _, err := repo.Exchange(ctx, id, usd, eur, 10, 9); !errors.Is(err, domain.ErrBalanceNotFound) {
				t.Fatalf("error = %v", err)
			}
			var total int64
			if err := pool.QueryRow(ctx, "SELECT sum(amount) FROM balances WHERE user_id=$1", id).Scan(&total); err != nil {
				t.Fatal(err)
			}
			want := int64(177)
			if missing == "USD" {
				want = 277
			}
			if total != want {
				t.Fatalf("remaining balances changed: %d; want %d", total, want)
			}
		})
	}
}
func TestExchangeRollbackAfterDebit(t *testing.T) {
	ctx, repo, pool := setupExchangeDatabase(t)
	id := seed(t, ctx, pool, 100, 200)
	// Reject only the credit UPDATE, after the debit has already succeeded.
	if _, err := pool.Exec(ctx, "ALTER TABLE balances ADD CONSTRAINT reject_credit CHECK (currency <> 'EUR' OR amount <= 200)"); err != nil {
		t.Fatal(err)
	}
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	if _, err := repo.Exchange(ctx, id, usd, eur, 10, 9); err == nil {
		t.Fatal("expected credit failure")
	}
	assertBalances(t, ctx, pool, id, 100, 200)
}
func TestExchangeConcurrent(t *testing.T) {
	for _, mode := range []string{"opposite directions", "cannot overdraw", "cannot overflow"} {
		t.Run(mode, func(t *testing.T) {
			ctx, repo, pool := setupExchangeDatabase(t)
			usdBalance, eurBalance := int64(100), int64(100)
			if mode == "cannot overdraw" {
				usdBalance = 10
			}
			if mode == "cannot overflow" {
				eurBalance = math.MaxInt64 - 10
			}
			id := seed(t, ctx, pool, usdBalance, eurBalance)
			usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
			eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
			start := make(chan struct{})
			results := make(chan error, 40)
			for i := 0; i < 40; i++ {
				go func(i int) {
					<-start
					from, to := usd, eur
					if mode == "opposite directions" && i%2 == 0 {
						from, to = to, from
					}
					_, err := repo.Exchange(ctx, id, from, to, 1, 1)
					results <- err
				}(i)
			}
			close(start)
			successes := 0
			for i := 0; i < 40; i++ {
				err := <-results
				if err == nil {
					successes++
					continue
				}
				want := domain.ErrSmallBalance
				if mode == "cannot overflow" {
					want = domain.ErrBalanceOverflow
				}
				if mode == "opposite directions" || !errors.Is(err, want) {
					t.Errorf("unexpected exchange error: %v", err)
				}
			}
			if mode == "opposite directions" {
				if successes != 40 {
					t.Errorf("successes = %d; want 40", successes)
				}
				assertBalances(t, ctx, pool, id, 100, 100)
			} else {
				if successes != 10 {
					t.Errorf("successes = %d; want 10", successes)
				}
				assertBalances(t, ctx, pool, id, usdBalance-10, eurBalance+10)
			}
		})
	}
}
func TestExchangeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := repository.NewPostgresRepository(nil)
	usd, _ := domain.NewCurrency(domain.CurrencyTypeUSD)
	eur, _ := domain.NewCurrency(domain.CurrencyTypeEUR)
	if _, err := repo.Exchange(ctx, uuid.New(), usd, eur, 1, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}
