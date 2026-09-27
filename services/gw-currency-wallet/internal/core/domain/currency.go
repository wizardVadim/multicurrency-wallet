package domain

type CurrencyType string

const (
	CurrencyTypeUSD CurrencyType = "USD"
	CurrencyTypeRUB CurrencyType = "RUB"
	CurrencyTypeEUR CurrencyType = "EUR"
)

type Currency struct {
	currencyType CurrencyType
}

func NewCurrency(currencyType CurrencyType) (Currency, error) {
	currency := Currency{
		currencyType: currencyType,
	}
	if err := currency.validate(); err != nil {
		return Currency{}, err
	}

	return currency, nil
}

func (currency Currency) validate() error {
	switch currency.currencyType {
	case CurrencyTypeEUR, CurrencyTypeUSD, CurrencyTypeRUB:
		return nil
	default:
		return ErrInvalidCurrencyType
	}
}

func (currency Currency) CurrencyType() CurrencyType {
	return currency.currencyType
}

func (currency Currency) IsEqual(other Currency) bool {
	return currency.currencyType == other.currencyType
}
