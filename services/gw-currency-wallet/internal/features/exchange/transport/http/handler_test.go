package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"wallet-app/internal/core/domain"
	exchangehttp "wallet-app/internal/features/exchange/transport/http"
)

type serviceStub func(context.Context) (domain.ExchangeRates, error)

func (s serviceStub) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	return s(ctx)
}

func TestGetExchangeRates(t *testing.T) {
	usd, err := domain.NewCurrency(domain.CurrencyTypeUSD)
	if err != nil {
		t.Fatal(err)
	}
	rates, err := domain.NewExchangeRates(usd, domain.Rates{"USD": 1, "RUB": 90, "EUR": 0.85})
	if err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "service error"
		}
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange/rates", nil)
			calls := 0
			handler := exchangehttp.New(serviceStub(func(ctx context.Context) (domain.ExchangeRates, error) {
				calls++
				if ctx != req.Context() {
					t.Error("request context not forwarded")
				}
				if fail {
					return domain.ExchangeRates{}, errors.New("private upstream details")
				}
				return rates, nil
			}))
			rec := httptest.NewRecorder()
			handler.GetExchangeRates(rec, req)
			if calls != 1 {
				t.Fatalf("service calls = %d", calls)
			}
			wantStatus := http.StatusOK
			wantJSON := `{"rates":{"USD":1,"RUB":90,"EUR":0.85}}`
			if fail {
				wantStatus = http.StatusInternalServerError
				wantJSON = `{"error":"Failed to retrieve exchange rates"}`
			}
			if rec.Code != wantStatus {
				t.Errorf("status = %d; want %d", rec.Code, wantStatus)
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Error("missing JSON content type")
			}
			var got, want any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("body = %s; want %s", rec.Body.String(), wantJSON)
			}
		})
	}
}

func TestGetExchangeRatesCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := exchangehttp.New(serviceStub(func(context.Context) (domain.ExchangeRates, error) {
		t.Fatal("service called after cancellation")
		return domain.ExchangeRates{}, nil
	}))
	rec := httptest.NewRecorder()
	handler.GetExchangeRates(rec, httptest.NewRequest(http.MethodGet, "/api/v1/exchange/rates", nil).WithContext(ctx))
	if rec.Body.Len() != 0 {
		t.Error("response written after cancellation")
	}
}
