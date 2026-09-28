package service

import (
	"testing"
	"time"
	"wallet-app/internal/core/domain"

	"github.com/google/uuid"
)

func TestPrepareLargeOperation(t *testing.T) {
	for _, opType := range []domain.OperationType{domain.OperationTypeDeposit, domain.OperationTypeWithdraw} {
		for _, tc := range []struct {
			name               string
			currency           domain.CurrencyType
			amount             int64
			rate               string
			wantEvent, wantErr bool
		}{
			{"small", domain.CurrencyTypeUSD, 99999, "1", false, false},
			{"usd_threshold", domain.CurrencyTypeUSD, 100000, "1", true, false},
			{"eur_threshold", domain.CurrencyTypeEUR, 87000, "0.87", true, false},
			{"rub_threshold", domain.CurrencyTypeRUB, 9010000, "90.1", true, false},
			{"invalid_rate", domain.CurrencyTypeUSD, 100000, "bad", false, true},
		} {
			t.Run(string(opType)+"/"+tc.name, func(t *testing.T) {
				currency, err := domain.NewCurrency(tc.currency)
				if err != nil {
					t.Fatal(err)
				}
				userID := uuid.New()
				operation, err := domain.NewBalanceOperation(userID, currency, opType, tc.amount)
				if err != nil {
					t.Fatal(err)
				}
				before := time.Now().UTC()
				event, err := prepareLargeOperation(operation, tc.rate)
				after := time.Now().UTC()
				if (err != nil) != tc.wantErr {
					t.Fatalf("error = %v; wantErr %v", err, tc.wantErr)
				}
				if (event != nil) != tc.wantEvent {
					t.Fatalf("event = %+v; wantEvent %v", event, tc.wantEvent)
				}
				if event == nil {
					return
				}
				if err := event.Validate(); err != nil {
					t.Fatalf("invalid event: %v", err)
				}
				if event.UserID != userID.String() || event.OperationType != string(opType) || event.Currency != string(tc.currency) || event.AmountMinor != tc.amount || event.UnitsPerUSD != tc.rate {
					t.Fatalf("incorrect event fields: %+v", event)
				}
				if event.EventID == event.TransactionID {
					t.Error("event and transaction IDs must be generated separately")
				}
				if event.OccurredAt.Before(before) || event.OccurredAt.After(after) {
					t.Error("timestamp is outside preparation interval")
				}
			})
		}
	}
}
