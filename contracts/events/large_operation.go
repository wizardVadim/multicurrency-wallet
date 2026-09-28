package events

import (
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var decimalRatePattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

type LargeOperation struct {
	SchemaVersion int       `json:"schema_version"`
	EventID       string    `json:"event_id"`
	TransactionID string    `json:"transaction_id"`
	UserID        string    `json:"user_id"`
	OperationType string    `json:"operation_type"`
	Status        string    `json:"status"`
	AmountMinor   int64     `json:"amount_minor"`
	Currency      string    `json:"currency"`
	UnitsPerUSD   string    `json:"units_per_usd"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func (e LargeOperation) Validate() error {
	if e.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schema_version %d", e.SchemaVersion)
	}
	if e.OperationType != "deposit" && e.OperationType != "withdraw" && e.OperationType != "exchange" {
		return fmt.Errorf("unsupported operation_type %q", e.OperationType)
	}
	if e.Status != "succeeded" {
		return fmt.Errorf("unsupported status %q", e.Status)
	}
	if e.AmountMinor <= 0 {
		return fmt.Errorf("unsupported amount_minor %d", e.AmountMinor)
	}
	if e.Currency != "USD" && e.Currency != "EUR" && e.Currency != "RUB" {
		return fmt.Errorf("unsupported currency %q", e.Currency)
	}
	_, offset := e.OccurredAt.Zone()
	if e.OccurredAt.IsZero() || offset != 0 {
		return fmt.Errorf("unsupported occurred_at %+v", e.OccurredAt)
	}
	eventUUID, err := uuid.Parse(e.EventID)
	if err != nil {
		return fmt.Errorf("%w: unsupported event_id %+v", err, e.EventID)
	}
	if eventUUID == uuid.Nil {
		return fmt.Errorf("unsupported event_id %+v", e.EventID)
	}
	transactionUUID, err := uuid.Parse(e.TransactionID)
	if err != nil {
		return fmt.Errorf("%w: unsupported transaction_id %+v", err, e.TransactionID)
	}
	if transactionUUID == uuid.Nil {
		return fmt.Errorf("unsupported transaction_id %+v", e.TransactionID)
	}
	userUUID, err := uuid.Parse(e.UserID)
	if err != nil {
		return fmt.Errorf("%w: unsupported user_id %+v", err, e.UserID)
	}
	if userUUID == uuid.Nil {
		return fmt.Errorf("unsupported user_id %+v", e.UserID)
	}
	if !decimalRatePattern.MatchString(e.UnitsPerUSD) {
		return fmt.Errorf("invalid units_per_usd: %q", e.UnitsPerUSD)
	}
	rate, ok := new(big.Rat).SetString(e.UnitsPerUSD)
	if !ok || rate.Sign() <= 0 {
		return fmt.Errorf("units_per_usd must be positive: %q", e.UnitsPerUSD)
	}
	if e.Currency == "USD" && e.UnitsPerUSD != "1" {
		return fmt.Errorf("units_per_usd must be \"1\" for USD: %q", e.UnitsPerUSD)
	}

	amount := new(big.Rat).SetInt64(e.AmountMinor)
	threshold := new(big.Rat).Mul(big.NewRat(100000, 1), rate)

	if amount.Cmp(threshold) < 0 {
		return fmt.Errorf("amount_minor is below the 1000 USD threshold")
	}

	return nil
}
