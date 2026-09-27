package app

import (
	"context"
	"contracts/exchange"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	exchangeclient "wallet-app/internal/features/exchange/client"
)

type stalledExchanger struct {
	exchange.UnimplementedExchangeServiceServer
	started  chan time.Time
	canceled chan struct{}
}

func (s *stalledExchanger) GetExchangeRates(ctx context.Context, _ *exchange.Empty) (*exchange.ExchangeRatesResponse, error) {
	deadline, _ := ctx.Deadline()
	s.started <- deadline
	<-ctx.Done()
	close(s.canceled)
	return nil, status.FromContextError(ctx.Err()).Err()
}

func TestExchangerDeadline(t *testing.T) {
	for _, shorterCaller := range []bool{false, true} {
		name := "configured timeout"
		if shorterCaller {
			name = "shorter caller deadline"
		}
		t.Run(name, func(t *testing.T) {
			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer()
			handler := &stalledExchanger{started: make(chan time.Time, 1), canceled: make(chan struct{})}
			exchange.RegisterExchangeServiceServer(server, handler)
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			t.Cleanup(func() {
				server.Stop()
				listener.Close()
				if err := <-done; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
					t.Errorf("Serve: %v", err)
				}
			})
			// Use the same connection factory as RunWithConfig, replacing only the transport.
			conn, err := newExchangerConnection("passthrough:///exchanger", time.Second,
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.Close() })
			callerTimeout := 5 * time.Second // Watchdog, longer than the configured RPC timeout.
			if shorterCaller {
				callerTimeout = 250 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), callerTimeout)
			defer cancel()
			started := time.Now()
			_, err = exchangeclient.New(exchange.NewExchangeServiceClient(conn)).GetExchangeRates(ctx)
			if status.Code(err) != codes.DeadlineExceeded {
				t.Fatalf("error = %v; want DeadlineExceeded", err)
			}
			select {
			case deadline := <-handler.started:
				want := time.Second
				if shorterCaller {
					want = callerTimeout
				}
				// Detect missing/wrong service configuration without strict scheduler timing assumptions.
				if got := deadline.Sub(started); got < want-100*time.Millisecond || got > want+100*time.Millisecond {
					t.Errorf("server deadline offset = %v; want approximately %v", got, want)
				}
			default:
				t.Fatal("RPC did not reach the stalled server")
			}
			if !shorterCaller && ctx.Err() != nil {
				t.Error("caller watchdog fired instead of RPC timeout")
			}
			select {
			case <-handler.canceled:
			case <-time.After(2 * time.Second):
				t.Fatal("server did not observe cancellation")
			}
		})
	}
}
