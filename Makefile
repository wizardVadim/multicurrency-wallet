WALLET_DIR := services/gw-currency-wallet
EXCHANGER_DIR := services/gw-exchanger
GO_PACKAGES := ./$(WALLET_DIR)/... ./$(EXCHANGER_DIR)/... ./contracts/...
COMPOSE := docker compose --env-file config.env
.DEFAULT_GOAL := local-run-go

.PHONY: local-run-go local-run-wallet local-run-exchanger
.PHONY: local-test-go test-wallet test-exchanger
.PHONY: docker-start docker-stop docker-rebuild db-wallet db-exchanger integration-test
.PHONY: load-test-add-balance load-test-get-balance load-test-minus-balance

# Backward-compatible wallet shortcut.
local-run-go: local-run-wallet

local-run-wallet:
	@POSTGRES_HOST=localhost go run ./$(WALLET_DIR)/cmd -c config.env

local-run-exchanger:
	@POSTGRES_HOST_EXCHANGER=localhost go run ./$(EXCHANGER_DIR)/cmd -c config.env

local-test-go:
	@go test $(GO_PACKAGES)

test-wallet:
	@go test ./$(WALLET_DIR)/...

test-exchanger:
	@go test ./$(EXCHANGER_DIR)/...

docker-start:
	@$(COMPOSE) up -d

docker-stop:
	@$(COMPOSE) down

docker-rebuild:
	@$(COMPOSE) up -d --build wallet exchanger

db-wallet:
	@$(COMPOSE) up -d --wait db-currency-wallet
	@$(COMPOSE) run --rm migrate-wallet

db-exchanger:
	@$(COMPOSE) up -d --wait db-exchanger
	@$(COMPOSE) run --rm migrate-exchanger

TEST_COMPOSE = docker compose -f docker-compose.test.yaml

integration-test:
	@set -eu; \
	export COMPOSE_PROJECT_NAME=wallet-tests-$$$$; \
	trap '$(TEST_COMPOSE) down --volumes' EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	$(TEST_COMPOSE) up --wait --wait-timeout 60; \
	test_address=$$($(TEST_COMPOSE) port db 5432); \
	TEST_DATABASE_URL="postgres://test_user:test_password@$$test_address/wallet_test?sslmode=disable" \
		go test -count=1 -v ./$(WALLET_DIR)/internal/features/wallet/repository ./$(WALLET_DIR)/internal/features/auth/repository ./$(WALLET_DIR)/internal/features/exchange/repository ./$(EXCHANGER_DIR)/internal/features/rates/repository

load-test-add-balance:
	@test -n "$$TOKEN" || { echo "Set TOKEN to a JWT returned by login" >&2; exit 1; }
	@cd $(WALLET_DIR) && vegeta attack -header="Authorization: Bearer $$TOKEN" -targets=./loadtests/vegeta_targets_add_balance.txt -rate=1000 -duration=30s | vegeta report

load-test-get-balance:
	@test -n "$$TOKEN" || { echo "Set TOKEN to a JWT returned by login" >&2; exit 1; }
	@cd $(WALLET_DIR) && vegeta attack -header="Authorization: Bearer $$TOKEN" -targets=./loadtests/vegeta_targets_get_balance.txt -rate=1000 -duration=30s | vegeta report

load-test-minus-balance:
	@test -n "$$TOKEN" || { echo "Set TOKEN to a JWT returned by login" >&2; exit 1; }
	@cd $(WALLET_DIR) && vegeta attack -header="Authorization: Bearer $$TOKEN" -targets=./loadtests/vegeta_targets_minus_balance.txt -rate=1000 -duration=30s | vegeta report
