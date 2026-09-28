package domain

import (
	"math/big"
	"regexp"
)

var decimalRatePattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func IsLargeOperation(amountMinor int64, unitsPerUSD string) (bool, error) {
	if amountMinor <= 0 {
		return false, ErrInvalidAmountMinor
	}
	amount := new(big.Rat).SetInt64(amountMinor)

	if !decimalRatePattern.MatchString(unitsPerUSD) {
		return false, ErrInvalidExchangeRateValue
	}

	rate, ok := new(big.Rat).SetString(unitsPerUSD)
	if !ok || rate.Sign() <= 0 {
		return false, ErrInvalidExchangeRateValue
	}

	threshold := new(big.Rat).Mul(big.NewRat(100000, 1), rate)

	return amount.Cmp(threshold) >= 0, nil
}
