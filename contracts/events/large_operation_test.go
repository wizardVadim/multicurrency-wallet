package events

import (
	"strings"
	"testing"
	"time"
)

func TestLargeOperationValidateBasicFields(t *testing.T) {
	valid := LargeOperation{
		SchemaVersion: 1,
		EventID:       "11111111-1111-4111-8111-111111111111",
		TransactionID: "22222222-2222-4222-8222-222222222222",
		UserID:        "33333333-3333-4333-8333-333333333333",
		OperationType: "deposit", Status: "succeeded",
		AmountMinor: 100000, Currency: "USD", UnitsPerUSD: "1",
		OccurredAt: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
	}
	cases := []struct {
		name   string
		change func(*LargeOperation)
		field  string
	}{
		{"deposit", func(e *LargeOperation) {}, ""},
		{"withdraw", func(e *LargeOperation) { e.OperationType = "withdraw" }, ""},
		{"exchange", func(e *LargeOperation) { e.OperationType = "exchange" }, ""},
		{"eur", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.87" }, ""},
		{"rub", func(e *LargeOperation) { e.Currency = "RUB"; e.UnitsPerUSD = "90.1"; e.AmountMinor = 9010000 }, ""},
		{"zero_offset_zone", func(e *LargeOperation) { e.OccurredAt = e.OccurredAt.In(time.FixedZone("zero", 0)) }, ""},
		{"event_id_empty", func(e *LargeOperation) { e.EventID = "" }, "event_id"},
		{"event_id_malformed", func(e *LargeOperation) { e.EventID = "not-a-uuid" }, "event_id"},
		{"event_id_nil", func(e *LargeOperation) { e.EventID = "00000000-0000-0000-0000-000000000000" }, "event_id"},
		{"transaction_id_empty", func(e *LargeOperation) { e.TransactionID = "" }, "transaction_id"},
		{"transaction_id_malformed", func(e *LargeOperation) { e.TransactionID = "not-a-uuid" }, "transaction_id"},
		{"transaction_id_nil", func(e *LargeOperation) { e.TransactionID = "00000000-0000-0000-0000-000000000000" }, "transaction_id"},
		{"user_id_empty", func(e *LargeOperation) { e.UserID = "" }, "user_id"},
		{"user_id_malformed", func(e *LargeOperation) { e.UserID = "not-a-uuid" }, "user_id"},
		{"user_id_nil", func(e *LargeOperation) { e.UserID = "00000000-0000-0000-0000-000000000000" }, "user_id"},
		{"invalid_rate_0", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "" }, "units_per_usd"},
		{"invalid_rate_1", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0" }, "units_per_usd"},
		{"invalid_rate_2", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.00" }, "units_per_usd"},
		{"invalid_rate_3", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "-1" }, "units_per_usd"},
		{"invalid_rate_4", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "+1" }, "units_per_usd"},
		{"invalid_rate_5", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = " 1" }, "units_per_usd"},
		{"invalid_rate_6", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1 " }, "units_per_usd"},
		{"invalid_rate_7", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1\n" }, "units_per_usd"},
		{"invalid_rate_8", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1/2" }, "units_per_usd"},
		{"invalid_rate_9", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1e2" }, "units_per_usd"},
		{"invalid_rate_10", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = ".5" }, "units_per_usd"},
		{"invalid_rate_11", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1." }, "units_per_usd"},
		{"invalid_rate_12", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "NaN" }, "units_per_usd"},
		{"invalid_rate_13", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "Inf" }, "units_per_usd"},
		{"invalid_rate_14", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0x10" }, "units_per_usd"},
		{"invalid_rate_15", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "1,5" }, "units_per_usd"},
		{"usd_rate_1.0", func(e *LargeOperation) { e.UnitsPerUSD = "1.0" }, "units_per_usd"},
		{"usd_rate_01", func(e *LargeOperation) { e.UnitsPerUSD = "01" }, "units_per_usd"},
		{"usd_rate_2", func(e *LargeOperation) { e.UnitsPerUSD = "2" }, "units_per_usd"},
		{"precise_decimal_rate", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.87000000000000000001" }, ""},
		{"threshold_USD_below", func(e *LargeOperation) { e.Currency = "USD"; e.UnitsPerUSD = "1"; e.AmountMinor = 99999 }, "amount_minor"},
		{"threshold_USD_equal", func(e *LargeOperation) { e.Currency = "USD"; e.UnitsPerUSD = "1"; e.AmountMinor = 100000 }, ""},
		{"threshold_USD_above", func(e *LargeOperation) { e.Currency = "USD"; e.UnitsPerUSD = "1"; e.AmountMinor = 100001 }, ""},
		{"threshold_EUR_below", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.87"; e.AmountMinor = 86999 }, "amount_minor"},
		{"threshold_EUR_equal", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.87"; e.AmountMinor = 87000 }, ""},
		{"threshold_EUR_above", func(e *LargeOperation) { e.Currency = "EUR"; e.UnitsPerUSD = "0.87"; e.AmountMinor = 87001 }, ""},
		{"threshold_RUB_below", func(e *LargeOperation) { e.Currency = "RUB"; e.UnitsPerUSD = "90.1"; e.AmountMinor = 9009999 }, "amount_minor"},
		{"threshold_RUB_equal", func(e *LargeOperation) { e.Currency = "RUB"; e.UnitsPerUSD = "90.1"; e.AmountMinor = 9010000 }, ""},
		{"threshold_RUB_above", func(e *LargeOperation) { e.Currency = "RUB"; e.UnitsPerUSD = "90.1"; e.AmountMinor = 9010001 }, ""},
		{"threshold_no_rounding", func(e *LargeOperation) {
			e.Currency = "EUR"
			e.UnitsPerUSD = "0.87000000000000000001"
			e.AmountMinor = 87000
		}, "amount_minor"},
		{"maximum_amount", func(e *LargeOperation) { e.AmountMinor = 9223372036854775807 }, ""},
		{"threshold_above_int64", func(e *LargeOperation) {
			e.Currency = "RUB"
			e.UnitsPerUSD = "9223372036854775807"
			e.AmountMinor = 9223372036854775807
		}, "amount_minor"},
		{"version", func(e *LargeOperation) { e.SchemaVersion = 2 }, "schema_version"},
		{"operation", func(e *LargeOperation) { e.OperationType = "transfer" }, "operation_type"},
		{"status", func(e *LargeOperation) { e.Status = "succeed" }, "status"},
		{"zero_amount", func(e *LargeOperation) { e.AmountMinor = 0 }, "amount_minor"},
		{"negative_amount", func(e *LargeOperation) { e.AmountMinor = -1 }, "amount_minor"},
		{"currency", func(e *LargeOperation) { e.Currency = "usd" }, "currency"},
		{"zero_time", func(e *LargeOperation) { e.OccurredAt = time.Time{} }, "occurred_at"},
		{"nonzero_offset", func(e *LargeOperation) { e.OccurredAt = e.OccurredAt.In(time.FixedZone("plus_one", 3600)) }, "occurred_at"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.change(&e)
			err := e.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatalf("valid event rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("expected error naming %s, got %v", tc.field, err)
			}
		})
	}
}
