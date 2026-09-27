package exchange_http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"wallet-app/internal/core/domain"
	authhttp "wallet-app/internal/features/auth/transport/http"
	exchangehttp "wallet-app/internal/features/exchange/transport/http"

	"github.com/google/uuid"
)

type exchangeStub struct {
	serviceStub
	exchange func(context.Context, uuid.UUID, domain.Currency, domain.Currency, int64) (domain.ExchangeResult, error)
}

func (s exchangeStub) Exchange(ctx context.Context, id uuid.UUID, from, to domain.Currency, amount int64) (domain.ExchangeResult, error) {
	return s.exchange(ctx, id, from, to, amount)
}

type validatorStub struct{ id uuid.UUID }

func (v validatorStub) Validate(string) (string, error) { return v.id.String(), nil }
func TestExchangeHTTPContract(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		serviceErr error
		status     int
		calls      int
	}{
		{"numeric amount", `{"from_currency":"USD","to_currency":"EUR","amount":100.00}`, nil, 200, 1},
		{"same currencies error", `{"from_currency":"USD","to_currency":"USD","amount":100.00}`, domain.ErrCurrenciesAreSame, 400, 1},
		{"insufficient funds", `{"from_currency":"USD","to_currency":"EUR","amount":100.00}`, domain.ErrSmallBalance, 400, 1},
		{"missing rate is upstream failure", `{"from_currency":"USD","to_currency":"EUR","amount":100.00}`, domain.ErrExchangeRateNotFound, 500, 1},
		{"invalid currency", `{"from_currency":"GBP","to_currency":"EUR","amount":100.00}`, nil, 400, 0},
		{"malformed body", `{`, nil, 400, 0},
		{"multiple values", `{} {}`, nil, 400, 0},
		{"too large", strings.Repeat(" ", 4097), nil, 413, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			calls := 0
			svc := exchangeStub{exchange: func(ctx context.Context, gotID uuid.UUID, from, to domain.Currency, amount int64) (domain.ExchangeResult, error) {
				calls++
				if gotID != id || amount != 10000 {
					t.Errorf("wrong service input: %v %d", gotID, amount)
				}
				if tt.serviceErr != nil {
					return domain.ExchangeResult{}, tt.serviceErr
				}
				return domain.NewExchangeResult(from, to, 2500, 8600, 8500)
			}}
			handler := authhttp.Authenticate(validatorStub{id}, http.HandlerFunc(exchangehttp.New(svc).Exchange))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/exchange", strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer test")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf("status = %d; want %d; body=%s", rec.Code, tt.status, rec.Body.String())
			}
			if calls != tt.calls {
				t.Errorf("service calls = %d; want %d", calls, tt.calls)
			}
			if tt.status == 200 && rec.Code == 200 {
				var got struct {
					Message  string                 `json:"message"`
					Amount   json.Number            `json:"exchanged_amount"`
					Balances map[string]json.Number `json:"new_balance"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Message != "Exchange successful" || got.Amount != "85.00" || got.Balances["USD"] != "25.00" || got.Balances["EUR"] != "86.00" || len(got.Balances) != 2 {
					t.Errorf("unexpected response: %s", rec.Body.String())
				}
			}
		})
	}
}
