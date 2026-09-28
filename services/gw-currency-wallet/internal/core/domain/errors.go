package domain

import "errors"

var (
	ErrInvalidOperationType = errors.New("invalid operation type")
)

var (
	ErrSmallBalance    = errors.New("insufficient funds on the balance")
	ErrBalanceOverflow = errors.New("balance overflow")
)

var (
	ErrInvalidUsername        = errors.New("invalid username")
	ErrInvalidEmailAddress    = errors.New("invalid email address")
	ErrInvalidUserID          = errors.New("invalid user id")
	ErrInvalidPasswordHash    = errors.New("invalid password hash")
	ErrUserNotFound           = errors.New("user is not found")
	ErrUsernameAlreadyExists  = errors.New("username already exists")
	ErrEmailAlreadyExists     = errors.New("email already exists")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrInvalidUserCredentials = errors.New("invalid user credentials")
	ErrPasswordMismatch       = errors.New("password mismatch")
)

var (
	ErrInvalidCurrencyType  = errors.New("invalid currency type")
	ErrInvalidBalanceAmount = errors.New("invalid balance amount")
	ErrBalanceNotFound      = errors.New("balance not found")
)

var (
	ErrInvalidExchangeRateValue = errors.New("invalid exchange rate value")
	ErrSameExchangeCurrency     = errors.New("exchange currencies must differ")
	ErrExchangeRatesNil         = errors.New("exchange rates is nil")
	ErrInvalidBaseCurrencyRate  = errors.New("invalid base currency rate")
	ErrBaseCurrencyNotConsists  = errors.New("base currency not consists")
)

var (
	ErrExchangeRateNotFound = errors.New("exchange rate not found")
	ErrCurrenciesAreSame    = errors.New("currencies are same")
)

var (
	ErrInvalidAmountMinor = errors.New("invalid amount minor")
)
