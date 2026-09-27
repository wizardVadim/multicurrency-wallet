package exchange_http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"wallet-app/internal/core/domain"
	auth_http "wallet-app/internal/features/auth/transport/http"
)

type Handler struct {
	exchangeService ExchangeService
}

func New(exchangeService ExchangeService) *Handler {
	return &Handler{
		exchangeService: exchangeService,
	}
}

// GET /api/v1/exchange/rates
func (h *Handler) GetExchangeRates(w http.ResponseWriter, r *http.Request) {
	if err := r.Context().Err(); err != nil {
		return
	}

	rates, err := h.exchangeService.GetExchangeRates(r.Context())
	if err != nil {
		status := http.StatusInternalServerError
		message := ErrorFailedRetrieveRates

		slog.ErrorContext(r.Context(), "get exchange rates", "error", err)
		writeJSON(w, status, ErrorResponse{
			Error: message,
		})
		return
	}

	var exchangeRatesResponse ExchangeRatesResponse
	exchangeRatesResponse.Rates = make(map[string]float32)
	for k, v := range rates.Rates() {
		exchangeRatesResponse.Rates[string(k)] = v
	}

	slog.InfoContext(r.Context(), "returned exchange rates")
	writeJSON(w, http.StatusOK, exchangeRatesResponse)
}

func writeJSON(w http.ResponseWriter, status int, response any) {
	body, err := json.Marshal(response)
	if err != nil {
		slog.Error("marshal response failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		slog.Warn("write response failed", "error", err)
	}
}

// POST /api/v1/exchange
//
// Request body:
//
//	{
//		"from_currency": "USD",
//		"to_currency": "EUR",
//		"amount": 100.00
//	}
func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	if err := r.Context().Err(); err != nil {
		return
	}

	userID, ok := auth_http.UserIDFromContext(r.Context())
	if !ok {
		if r.Context().Err() != nil {
			return
		}
		slog.ErrorContext(r.Context(), "exchange balance failed: couldn't find user ID in context", "error", domain.ErrInvalidUserID)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: ErrorInternalServerError})
		return
	}

	var exchangeDTO ExchangeDTO

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&exchangeDTO)
	if err == nil {
		var extra any
		if nextErr := decoder.Decode(&extra); nextErr != io.EOF {
			if nextErr == nil {
				err = errors.New("multiple JSON values")
			} else {
				err = nextErr
			}
		}
	}
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		slog.WarnContext(r.Context(), "exchange balance", "error", err)
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: ErrorRequestBodyTooLarge})
			return
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: ErrorInvalidRequestBody})
		return
	}

	fromCurrency, err := domain.NewCurrency(domain.CurrencyType(exchangeDTO.FromCurrency))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCurrencyType) {
			slog.WarnContext(r.Context(), "exchange balance", "error", err, "user id", userID, "from currency", exchangeDTO.FromCurrency)
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: ErrorInsufficientFundsOrInvalidCurrencies})
			return
		}
		slog.ErrorContext(r.Context(), "exchange balance", "error", err, "user id", userID)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: ErrorInternalServerError})
		return
	}

	toCurrency, err := domain.NewCurrency(domain.CurrencyType(exchangeDTO.ToCurrency))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCurrencyType) {
			slog.WarnContext(r.Context(), "exchange balance", "error", err, "user id", userID, "to currency", exchangeDTO.ToCurrency)
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: ErrorInsufficientFundsOrInvalidCurrencies})
			return
		}
		slog.ErrorContext(r.Context(), "exchange balance", "error", err, "user id", userID)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: ErrorInternalServerError})
		return
	}

	amount, err := jsonNumberToAmountInt64(exchangeDTO.Amount)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		slog.WarnContext(r.Context(), "exchange balance failed", "error", err, "user id", userID, "amount", exchangeDTO.Amount)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: ErrorInsufficientFundsOrInvalidCurrencies})
		return
	}

	exchangeResult, err := h.exchangeService.Exchange(r.Context(), userID, fromCurrency, toCurrency, amount)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		var errorMessage ErrorMessage
		var statusCode int
		if errors.Is(err, domain.ErrInvalidBalanceAmount) ||
			errors.Is(err, domain.ErrBalanceOverflow) ||
			errors.Is(err, domain.ErrSmallBalance) ||
			errors.Is(err, domain.ErrCurrenciesAreSame) ||
			errors.Is(err, domain.ErrBalanceOverflow) {
			errorMessage = ErrorInsufficientFundsOrInvalidCurrencies
			statusCode = http.StatusBadRequest
		} else {
			errorMessage = ErrorInternalServerError
			statusCode = http.StatusInternalServerError
		}
		if statusCode == 400 {
			slog.WarnContext(r.Context(), "exchange balance failed", "error", err, "user id", userID, "amount", exchangeDTO.Amount)
		} else {
			slog.ErrorContext(r.Context(), "exchange balance failed", "error", err, "user id", userID, "amount", exchangeDTO.Amount)
		}
		writeJSON(w, statusCode, ErrorResponse{Error: errorMessage})
		return
	}

	fromCurrencyResult := exchangeResult.FromCurrency().CurrencyType()
	toCurrencyResult := exchangeResult.ToCurrency().CurrencyType()
	fromAmountResult := exchangeResult.FromBalanceAmount()
	toAmountResult := exchangeResult.ToBalanceAmount()
	exchangedAmountResult := exchangeResult.ExchangedAmount()

	newBalance := map[string]json.Number{
		string(fromCurrencyResult): amountInt64ToJsonNumber(fromAmountResult),
		string(toCurrencyResult):   amountInt64ToJsonNumber(toAmountResult),
	}

	exchangeResponse := ExchangeResponse{
		Message:         "Exchange successful",
		ExchangedAmount: amountInt64ToJsonNumber(exchangedAmountResult),
		NewBalance:      newBalance,
	}

	slog.InfoContext(r.Context(), "exchanged balance", "user id", userID)
	writeJSON(w, http.StatusOK, exchangeResponse)
}

func jsonNumberToAmountInt64(input json.Number) (int64, error) {
	if strings.ContainsAny(input.String(), "eE") {
		return 0, fmt.Errorf("%w: %+v", errInvalidInputAmount, input)
	}

	parts := strings.Split(input.String(), ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("%w: %+v", errInvalidInputAmount, input)
	}
	if strings.TrimSpace(parts[0]) == "" {
		return 0, fmt.Errorf("%w: %+v", errInvalidInputAmount, input)
	}
	if len(parts) == 2 && len(parts[1]) > 2 {
		return 0, fmt.Errorf("%w: %+v", errInvalidInputAmount, input)
	}
	if len(parts) == 2 {
		if len(parts[1]) == 1 {
			parts[1] = parts[1] + "0"
		} else if len(parts[1]) == 0 {
			parts[1] = parts[1] + "00"
		}
	}
	if len(parts) == 1 {
		parts = append(parts, "00")
	}
	amount, err := strconv.ParseInt(strings.Join(parts, ""), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("convert json number to int 64: %w: %+v", errors.Join(err, errInvalidInputAmount), input)
	}
	return amount, nil
}

func amountInt64ToJsonNumber(amount int64) json.Number {
	return json.Number(
		fmt.Sprintf("%d.%02d", amount/100, amount%100),
	)
}
