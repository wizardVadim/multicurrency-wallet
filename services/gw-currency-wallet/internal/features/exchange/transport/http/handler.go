package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
