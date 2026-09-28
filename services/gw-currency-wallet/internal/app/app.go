package app

import (
	"context"
	"contracts/exchange"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wallet-app/internal/core/config"
	"wallet-app/internal/core/infrastructure/hash"
	"wallet-app/internal/core/infrastructure/id"
	"wallet-app/internal/core/infrastructure/token"
	auth_repository "wallet-app/internal/features/auth/repository"
	auth_service "wallet-app/internal/features/auth/service"
	auth_http "wallet-app/internal/features/auth/transport/http"
	"wallet-app/internal/features/wallet/repository"
	"wallet-app/internal/features/wallet/service"
	wallet_http "wallet-app/internal/features/wallet/transport/http"

	exchange_cache "wallet-app/internal/features/exchange/cache"
	exchange_client "wallet-app/internal/features/exchange/client"
	exchange_repository "wallet-app/internal/features/exchange/repository"
	exchange_service "wallet-app/internal/features/exchange/service"
	exchange_http "wallet-app/internal/features/exchange/transport/http"

	docs "wallet-app/docs"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})).With("service", "gw-currency-wallet")
	return RunWithConfig(cfg, logger)
}

func RunWithConfig(config config.Config, logger *slog.Logger) error {
	slog.SetDefault(logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("wallet starting")

	connectionString := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(config.DB.User, config.DB.Password),
		Host:   net.JoinHostPort(config.DB.Host, config.DB.Port),
		Path:   "/" + config.DB.Name,
	}

	poolConfig, err := pgxpool.ParseConfig(connectionString.String())
	if err != nil {
		return errors.New("invalid database connection configuration")
	}

	poolConfig.MaxConns = int32(config.MaxDbConnections)
	poolConfig.MinConns = int32(config.MinDbConnections)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	logger.Debug("database connection established")

	hasher := hash.New()
	tokenGenerator, err := token.NewJWTGenerator(config.Authorization.JwtSecretKey, time.Hour*time.Duration(config.Authorization.JwtTTL))
	if err != nil {
		return fmt.Errorf("%w: invalid token generator config", err)
	}

	grpcRequestTimeout := time.Second * time.Duration(config.Exchanger.ExchangerRequestTimeout)
	exchangerConnection, err := newExchangerConnection(config.Exchanger.ExchangerGrpcAddr, grpcRequestTimeout)
	if err != nil {
		return fmt.Errorf("exchanger connection: %w", err)
	}
	defer exchangerConnection.Close()

	exchangerClient := exchange_client.New(exchange.NewExchangeServiceClient(exchangerConnection))

	walletRepository := repository.NewPostgresRepository(pool)
	authRepository := auth_repository.NewPostgresRepository(pool)

	ratesCache := exchange_cache.NewMemory(
		exchangerClient,
		time.Duration(config.ExchangeRatesCacheTTL)*time.Second,
	)

	walletService := service.New(walletRepository)
	authService := auth_service.New(authRepository, id.GenerateUUID, hasher, tokenGenerator)
	exchangeService := exchange_service.New(ratesCache, exchange_repository.NewPostgresRepository(pool))

	exchangeHandler := exchange_http.New(exchangeService)
	walletHandler := wallet_http.New(walletService)
	authHandler := auth_http.New(authService)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(docs.OpenAPI)
	})

	mux.HandleFunc("GET /swagger/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(docs.SwaggerHTML)
	})

	mux.Handle("GET /api/v1/exchange/rates", auth_http.Authenticate(tokenGenerator, http.HandlerFunc(exchangeHandler.GetExchangeRates)))
	mux.Handle("POST /api/v1/exchange", auth_http.Authenticate(tokenGenerator, http.HandlerFunc(exchangeHandler.Exchange)))
	mux.HandleFunc("POST /api/v1/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/login", authHandler.Login)
	mux.Handle("GET /api/v1/balance", auth_http.Authenticate(tokenGenerator, http.HandlerFunc(walletHandler.GetBalances)))
	mux.Handle("POST /api/v1/wallet/deposit", auth_http.Authenticate(tokenGenerator, http.HandlerFunc(walletHandler.BalanceDeposit)))
	mux.Handle("POST /api/v1/wallet/withdraw", auth_http.Authenticate(tokenGenerator, http.HandlerFunc(walletHandler.BalanceWithdraw)))

	server := &http.Server{
		Addr:              ":" + config.HTTPPort,
		Handler:           wallet_http.Logging(logger, mux),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadHeaderTimeout: time.Duration(config.ReadHeaderTimeout) * time.Second,
		ReadTimeout:       time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(config.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(config.IdleTimeout) * time.Second,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}
	defer listener.Close()
	logger.Info("http server listening", "address", listener.Addr().String())

	stopCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http server: %w", err)

	case <-stopCtx.Done():
		logger.Info("http shutdown started")
		stop()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Warn("graceful shutdown failed; closing active connections")
			_ = server.Close()
			return fmt.Errorf("shutdown http server: %w", err)
		}

		logger.Info("http shutdown completed")
		return nil
	}
}
