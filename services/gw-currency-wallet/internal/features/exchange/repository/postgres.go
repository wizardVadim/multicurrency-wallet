package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PostgresRepository struct {
	db DBTX
}

func NewPostgresRepository(db DBTX) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

func (repo *PostgresRepository) Exchange(ctx context.Context, userID uuid.UUID, from, to domain.Currency, fromAmount, toAmount int64) (domain.ExchangeResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ExchangeResult{}, err
	}

	selectForUpdateQuery := `
		SELECT amount FROM balances WHERE user_id = $1 AND currency = $2 FOR UPDATE;
	`

	updateQuery := `
		UPDATE balances SET amount = amount %s $1 WHERE user_id = $2 AND currency = $3
	`

	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return domain.ExchangeResult{}, fmt.Errorf("exchange: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			if errors.Is(err, pgx.ErrTxClosed) {
				return
			}
			slog.ErrorContext(ctx, "exchange: rollback error", "error", err)
		}
	}()

	var currentFromAmount, currentToAmount int64

	firstCurrency, secondCurrency := from.CurrencyType(), to.CurrencyType()
	firstAmount, secondAmount := &currentFromAmount, &currentToAmount

	if firstCurrency > secondCurrency {
		firstCurrency, secondCurrency = secondCurrency, firstCurrency
		firstAmount, secondAmount = secondAmount, firstAmount
	}

	if err := tx.QueryRow(ctx, selectForUpdateQuery, userID, firstCurrency).
		Scan(firstAmount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExchangeResult{}, domain.ErrBalanceNotFound
		}
		return domain.ExchangeResult{}, fmt.Errorf("exchange: lock first balance: %w", err)
	}

	if err := tx.QueryRow(ctx, selectForUpdateQuery, userID, secondCurrency).
		Scan(secondAmount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExchangeResult{}, domain.ErrBalanceNotFound
		}
		return domain.ExchangeResult{}, fmt.Errorf("exchange: lock second balance: %w", err)
	}

	if currentFromAmount < fromAmount {
		return domain.ExchangeResult{}, fmt.Errorf("%w: got = %v; want withdraw = %v", domain.ErrSmallBalance, currentFromAmount, fromAmount)
	}
	if currentToAmount > math.MaxInt64-toAmount {
		return domain.ExchangeResult{}, fmt.Errorf("%w: got = %v; want deposit = %v", domain.ErrBalanceOverflow, currentToAmount, toAmount)
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(updateQuery, "-"), fromAmount, userID, from.CurrencyType()); err != nil {
		return domain.ExchangeResult{}, fmt.Errorf("exchange: %w", err)
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(updateQuery, "+"), toAmount, userID, to.CurrencyType()); err != nil {
		return domain.ExchangeResult{}, fmt.Errorf("exchange: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ExchangeResult{}, fmt.Errorf("exchange: %w", err)
	}

	exchangeResult, err := domain.NewExchangeResult(from, to, currentFromAmount-fromAmount, currentToAmount+toAmount, toAmount)
	if err != nil {
		return domain.ExchangeResult{}, err
	}

	return exchangeResult, nil
}
