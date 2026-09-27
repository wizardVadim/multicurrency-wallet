package config

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ExchangeRatesCacheTTL int // Seconds.
	LogLevel              slog.Level
	HTTPPort              string
	DB                    DBConfig
	MaxDbConnections      int
	MinDbConnections      int
	ReadHeaderTimeout     int
	ReadTimeout           int
	WriteTimeout          int
	IdleTimeout           int
	Authorization         AuthConfig
	Exchanger             ExchangerConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type AuthConfig struct {
	JwtSecretKey string
	JwtTTL       int
}

type ExchangerConfig struct {
	ExchangerGrpcAddr       string
	ExchangerRequestTimeout int
}

// LoadFile reads dotenv settings; explicitly set environment variables take precedence.
func LoadFile(path string) (Config, error) {
	values, err := godotenv.Read(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot read or parse configuration file %q", path)
	}
	return load(func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return values[key]
	}, true)
}

func Load() (Config, error) { return load(os.Getenv, false) }

func load(getenv func(string) string, isLocalhost bool) (Config, error) {
	required := []string{
		"HTTP_PORT",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_NAME",
		"MAX_DB_CONNECTIONS",
		"MIN_DB_CONNECTIONS",
		"READ_HEADER_TIMEOUT",
		"READ_TIMEOUT",
		"WRITE_TIMEOUT",
		"IDLE_TIMEOUT",
		"JWT_SECRET_KEY",
		"JWT_TTL",
		"EXCHANGER_GRPC_DOCKER_ADDR",
		"EXCHANGER_GRPC_LOCALHOST_ADDR",
		"EXCHANGER_TIMEOUT",
		"EXCHANGE_RATES_CACHE_TTL",
	}

	for _, key := range required {
		if getenv(key) == "" {
			return Config{}, fmt.Errorf("environment variable %s is required", key)
		}
	}

	maxDbConnections, err := strconv.Atoi(getenv("MAX_DB_CONNECTIONS"))
	if err != nil {
		return Config{}, fmt.Errorf("MAX_DB_CONNECTIONS: cannot convert variable: %w", err)
	}
	minDbConnections, err := strconv.Atoi(getenv("MIN_DB_CONNECTIONS"))
	if err != nil {
		return Config{}, fmt.Errorf("MIN_DB_CONNECTIONS: cannot convert variable: %w", err)
	}
	readHeaderTimeout, err := strconv.Atoi(getenv("READ_HEADER_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("READ_HEADER_TIMEOUT: cannot convert variable: %w", err)
	}
	readTimeout, err := strconv.Atoi(getenv("READ_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("READ_TIMEOUT: cannot convert variable: %w", err)
	}
	writeTimeout, err := strconv.Atoi(getenv("WRITE_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("WRITE_TIMEOUT: cannot convert variable: %w", err)
	}
	idleTimeout, err := strconv.Atoi(getenv("IDLE_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("IDLE_TIMEOUT: cannot convert variable: %w", err)
	}

	jwtTTL, err := strconv.Atoi(getenv("JWT_TTL"))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL: cannot convert variable: %w", err)
	}

	exchangerTimeout, err := strconv.Atoi(getenv("EXCHANGER_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("EXCHANGER_TIMEOUT: cannot convert variable: %w", err)
	}

	ratesCacheTTL, err := strconv.Atoi(getenv("EXCHANGE_RATES_CACHE_TTL"))
	if err != nil {
		return Config{}, fmt.Errorf("EXCHANGE_RATES_CACHE_TTL: cannot convert variable: %w", err)
	}

	var exchangerGrpcAddr string
	if isLocalhost {
		exchangerGrpcAddr = getenv("EXCHANGER_GRPC_LOCALHOST_ADDR")
	} else {
		exchangerGrpcAddr = getenv("EXCHANGER_GRPC_DOCKER_ADDR")
	}

	config := Config{
		ExchangeRatesCacheTTL: ratesCacheTTL,
		HTTPPort:              getenv("HTTP_PORT"),
		MaxDbConnections:      maxDbConnections,
		MinDbConnections:      minDbConnections,
		ReadHeaderTimeout:     readHeaderTimeout,
		ReadTimeout:           readTimeout,
		WriteTimeout:          writeTimeout,
		IdleTimeout:           idleTimeout,
		DB: DBConfig{
			Host:     getenv("POSTGRES_HOST"),
			Port:     getenv("POSTGRES_PORT"),
			User:     getenv("POSTGRES_USER"),
			Password: getenv("POSTGRES_PASSWORD"),
			Name:     getenv("POSTGRES_NAME"),
		},
		Authorization: AuthConfig{
			JwtSecretKey: getenv("JWT_SECRET_KEY"),
			JwtTTL:       jwtTTL,
		},
		Exchanger: ExchangerConfig{
			ExchangerGrpcAddr:       exchangerGrpcAddr,
			ExchangerRequestTimeout: exchangerTimeout,
		},
	}

	if level := getenv("LOG_LEVEL_WALLET"); level != "" {
		if err := config.LogLevel.UnmarshalText([]byte(level)); err != nil {
			return Config{}, fmt.Errorf("LOG_LEVEL_WALLET must be DEBUG, INFO, WARN or ERROR")
		}
	}

	if err := config.validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (config Config) validate() error {
	if config.ExchangeRatesCacheTTL < 1 || int64(config.ExchangeRatesCacheTTL) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("EXCHANGE_RATES_CACHE_TTL error: %v", errInvalidSettingValue)
	}
	if config.MaxDbConnections < 1 || config.MaxDbConnections > math.MaxInt32 {
		return fmt.Errorf("MAX_DB_CONNECTIONS error: %v", errInvalidSettingValue)
	}
	if config.MinDbConnections < 0 || config.MinDbConnections > config.MaxDbConnections {
		return fmt.Errorf("MIN_DB_CONNECTIONS error: %v", errInvalidSettingValue)
	}
	if config.ReadHeaderTimeout < 1 || int64(config.ReadHeaderTimeout) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("READ_HEADER_TIMEOUT error: %v", errInvalidSettingValue)
	}
	if config.ReadTimeout < 1 || int64(config.ReadTimeout) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("READ_TIMEOUT error: %v", errInvalidSettingValue)
	}
	if config.WriteTimeout < 1 || int64(config.WriteTimeout) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("WRITE_TIMEOUT error: %v", errInvalidSettingValue)
	}
	if config.IdleTimeout < 1 || int64(config.IdleTimeout) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("IDLE_TIMEOUT error: %v", errInvalidSettingValue)
	}
	if config.Authorization.JwtTTL < 1 || int64(config.Authorization.JwtTTL) > math.MaxInt64/int64(time.Hour) {
		return fmt.Errorf("JWT_TTL error: %v", errInvalidSettingValue)
	}
	if config.Exchanger.ExchangerRequestTimeout < 1 || int64(config.Exchanger.ExchangerRequestTimeout) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("EXCHANGER_TIMEOUT error: %v", errInvalidSettingValue)
	}
	return nil
}
