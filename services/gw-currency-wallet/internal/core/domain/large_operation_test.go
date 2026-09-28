package domain

import (
	"errors"
	"math"
	"testing"
)

func TestIsLargeOperation(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		rate   string
		want   bool
		err    error
	}{
		{"usd_below", 99999, "1", false, nil},
		{"usd_equal", 100000, "1", true, nil},
		{"usd_above", 100001, "1", true, nil},
		{"eur_below", 86999, "0.87", false, nil},
		{"eur_equal", 87000, "0.87", true, nil},
		{"eur_above", 87001, "0.87", true, nil},
		{"rub_below", 9009999, "90.1", false, nil},
		{"rub_equal", 9010000, "90.1", true, nil},
		{"rub_above", 9010001, "90.1", true, nil},
		{"no_rounding", 87000, "0.87000000000000000001", false, nil},
		{"maximum_amount", math.MaxInt64, "1", true, nil},
		{"threshold_above_int64", math.MaxInt64, "9223372036854775807", false, nil},
		{"zero_amount", 0, "1", false, ErrInvalidAmountMinor},
		{"negative_amount", -1, "1", false, ErrInvalidAmountMinor},
	}
	for _, rate := range []string{"", "0", "0.00", "-1", "abc", "NaN", "Inf", "1/2", "1e2", "+1", ".5", "1.", " 1", "1 ", "0x10"} {
		cases = append(cases, struct {
			name   string
			amount int64
			rate   string
			want   bool
			err    error
		}{"invalid_rate_" + rate, 100000, rate, false, ErrInvalidExchangeRateValue})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsLargeOperation(tc.amount, tc.rate)
			if got != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("IsLargeOperation(%d, %q) = %v, %v; want %v, %v", tc.amount, tc.rate, got, err, tc.want, tc.err)
			}
		})
	}
}
