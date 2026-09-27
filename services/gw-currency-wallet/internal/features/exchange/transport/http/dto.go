package exchange_http

import "encoding/json"

type ErrorMessage string

const (
	ErrorFailedRetrieveRates                  ErrorMessage = "Failed to retrieve exchange rates"
	ErrorInternalServerError                  ErrorMessage = "Internal server error"
	ErrorRequestBodyTooLarge                  ErrorMessage = "Request body too large"
	ErrorInvalidRequestBody                   ErrorMessage = "Invalid request body"
	ErrorInsufficientFundsOrInvalidCurrencies ErrorMessage = "Insufficient funds or invalid currencies"
)

type ErrorResponse struct {
	Error ErrorMessage `json:"error"`
}

type ExchangeRatesResponse struct {
	Rates map[string]float32 `json:"rates"`
}

type ExchangeDTO struct {
	FromCurrency string      `json:"from_currency"`
	ToCurrency   string      `json:"to_currency"`
	Amount       json.Number `json:"amount"`
}

type ExchangeResponse struct {
	Message         string                 `json:"message"`
	ExchangedAmount json.Number            `json:"exchanged_amount"`
	NewBalance      map[string]json.Number `json:"new_balance"`
}
