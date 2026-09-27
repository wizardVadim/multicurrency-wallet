package http

type ErrorMessage string

const (
	ErrorFailedRetrieveRates ErrorMessage = "Failed to retrieve exchange rates"
)

type ErrorResponse struct {
	Error ErrorMessage `json:"error"`
}

type ExchangeRatesResponse struct {
	Rates map[string]float32 `json:"rates"`
}
