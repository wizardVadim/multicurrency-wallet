package config_test

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"wallet-app/internal/core/config"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"EXCHANGE_RATES_CACHE_TTL":      "30",
		"LOG_LEVEL_WALLET":              "INFO",
		"EXCHANGER_GRPC_DOCKER_ADDR":    "exchanger:50051",
		"EXCHANGER_GRPC_LOCALHOST_ADDR": "localhost:50051",
		"EXCHANGER_TIMEOUT":             "5",
		"JWT_SECRET_KEY":                "0123456789abcdef0123456789abcdef",
		"JWT_TTL":                       "24",
		"HTTP_PORT":                     "8080",
		"MAX_DB_CONNECTIONS":            "1",
		"MIN_DB_CONNECTIONS":            "1",
		"READ_HEADER_TIMEOUT":           "5",
		"READ_TIMEOUT":                  "5",
		"WRITE_TIMEOUT":                 "20",
		"IDLE_TIMEOUT":                  "120",
		"POSTGRES_HOST":                 "localhost",
		"POSTGRES_PORT":                 "5432",
		"POSTGRES_USER":                 "test_user",
		"POSTGRES_PASSWORD":             "test_password",
		"POSTGRES_NAME":                 "wallet_test",
	} {
		t.Setenv(key, value)
	}
}

func TestLoad(t *testing.T) {
	setValidEnv(t)

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Config{
		ExchangeRatesCacheTTL: 30,
		Exchanger:             config.ExchangerConfig{ExchangerGrpcAddr: "exchanger:50051", ExchangerRequestTimeout: 5},
		Authorization:         config.AuthConfig{JwtSecretKey: "0123456789abcdef0123456789abcdef", JwtTTL: 24},
		HTTPPort:              "8080",
		MaxDbConnections:      1,
		MinDbConnections:      1,
		ReadHeaderTimeout:     5,
		ReadTimeout:           5,
		WriteTimeout:          20,
		IdleTimeout:           120,
		DB: config.DBConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "test_user",
			Password: "test_password",
			Name:     "wallet_test",
		},
	}
	if got != want {
		t.Fatalf("Load() = %+v; want %+v", got, want)
	}
}

func TestLoadRequiredEnv(t *testing.T) {
	keys := []string{
		"JWT_SECRET_KEY", "JWT_TTL", "HTTP_PORT",
		"EXCHANGER_GRPC_DOCKER_ADDR", "EXCHANGER_GRPC_LOCALHOST_ADDR", "EXCHANGER_TIMEOUT", "EXCHANGE_RATES_CACHE_TTL",
		"MAX_DB_CONNECTIONS",
		"MIN_DB_CONNECTIONS",
		"READ_HEADER_TIMEOUT",
		"READ_TIMEOUT",
		"WRITE_TIMEOUT",
		"IDLE_TIMEOUT",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_NAME",
	}

	for _, key := range keys {
		for _, state := range []string{"empty", "unset"} {
			t.Run(key+"/"+state, func(t *testing.T) {
				setValidEnv(t)
				if state == "empty" {
					t.Setenv(key, "")
				} else {
					if err := os.Unsetenv(key); err != nil {
						t.Fatalf("Unsetenv(%q): %v", key, err)
					}
				}

				got, err := config.Load()
				wantErr := fmt.Sprintf("environment variable %s is required", key)
				if err == nil || err.Error() != wantErr {
					t.Fatalf("Load() error = %v; want %q", err, wantErr)
				}
				if got != (config.Config{}) {
					t.Fatalf("Load() returned nonzero config on error: %+v", got)
				}
			})
		}
	}
}

func TestLoadInvalidNumericSettings(t *testing.T) {
	tests := []struct{ key, value string }{
		{"MAX_DB_CONNECTIONS", "0"},
		{"MAX_DB_CONNECTIONS", "-1"},
		{"MAX_DB_CONNECTIONS", "2147483648"},
		{"MIN_DB_CONNECTIONS", "-1"},
		{"MIN_DB_CONNECTIONS", "2"},
	}
	keys := []string{"MAX_DB_CONNECTIONS", "MIN_DB_CONNECTIONS", "READ_HEADER_TIMEOUT", "READ_TIMEOUT", "WRITE_TIMEOUT", "IDLE_TIMEOUT", "EXCHANGER_TIMEOUT", "EXCHANGE_RATES_CACHE_TTL"}
	for _, key := range keys {
		for _, value := range []string{"abc", "1.5", "999999999999999999999999999"} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, key := range keys[2:] {
		for _, value := range []string{"0", "-1", strconv.FormatInt(math.MaxInt64/int64(time.Second)+1, 10)} {
			tests = append(tests, struct{ key, value string }{key, value})
		}
	}
	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.value, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.key, tt.value)
			got, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("Load() error = %v; want error identifying %s", err, tt.key)
			}
			if got != (config.Config{}) {
				t.Fatal("expected empty config on error")
			}
		})
	}
}

func TestLoadNumericBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name   string
		values map[string]string
	}{
		{"zero minimum", map[string]string{"MIN_DB_CONNECTIONS": "0"}},
		{"equal pool limits", map[string]string{"MAX_DB_CONNECTIONS": "4", "MIN_DB_CONNECTIONS": "4"}},
		{"maximum pool size", map[string]string{"MAX_DB_CONNECTIONS": "2147483647", "MIN_DB_CONNECTIONS": "2147483647"}},
		{"minimum timeouts", map[string]string{"READ_HEADER_TIMEOUT": "1", "READ_TIMEOUT": "1", "WRITE_TIMEOUT": "1", "IDLE_TIMEOUT": "1", "EXCHANGER_TIMEOUT": "1", "EXCHANGE_RATES_CACHE_TTL": "1"}},
		{"maximum timeouts", map[string]string{"READ_HEADER_TIMEOUT": "9223372036", "READ_TIMEOUT": "9223372036", "WRITE_TIMEOUT": "9223372036", "IDLE_TIMEOUT": "9223372036", "EXCHANGER_TIMEOUT": "9223372036", "EXCHANGE_RATES_CACHE_TTL": "9223372036"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			for key, value := range tt.values {
				t.Setenv(key, value)
			}
			got, err := config.Load()
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			actual := map[string]int{
				"EXCHANGE_RATES_CACHE_TTL": got.ExchangeRatesCacheTTL,
				"MAX_DB_CONNECTIONS":       got.MaxDbConnections, "MIN_DB_CONNECTIONS": got.MinDbConnections,
				"READ_HEADER_TIMEOUT": got.ReadHeaderTimeout, "READ_TIMEOUT": got.ReadTimeout,
				"WRITE_TIMEOUT": got.WriteTimeout, "IDLE_TIMEOUT": got.IdleTimeout,
				"EXCHANGER_TIMEOUT": got.Exchanger.ExchangerRequestTimeout,
			}
			for key, value := range tt.values {
				if strconv.Itoa(actual[key]) != value {
					t.Errorf("%s = %d; want %s", key, actual[key], value)
				}
			}
		})
	}
}

func TestJWTLifetime(t *testing.T) {
	maxHours := math.MaxInt64 / int64(time.Hour)
	for _, tt := range []struct {
		value string
		valid bool
	}{
		{"1", true}, {"24", true}, {strconv.FormatInt(maxHours, 10), true},
		{"0", false}, {"-1", false}, {"abc", false}, {"1.5", false}, {"24h", false},
		{strconv.FormatInt(maxHours+1, 10), false}, {"9999999999999999999999999", false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv("JWT_TTL", tt.value)
			got, err := config.Load()
			if tt.valid {
				if err != nil {
					t.Fatal(err)
				}
				if strconv.Itoa(got.Authorization.JwtTTL) != tt.value {
					t.Error("incorrect JWT TTL")
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "JWT_TTL") {
					t.Errorf("error = %v; want JWT_TTL error", err)
				}
				if got != (config.Config{}) {
					t.Error("nonzero config on error")
				}
			}
		})
	}
}
