# gw-currency-wallet

A wallet REST API built with Go and PostgreSQL. Deposits and withdrawals use
atomic SQL updates, with balance checks in the same statement to prevent lost
updates, negative balances, and integer overflow under concurrent requests. Currency
exchange locks both balances in a fixed order and applies debit and credit in
one transaction.

Unless stated otherwise, run commands from the repository root (two levels above this directory).
See the [repository README](../../README.md) for the workspace layout.

## Quick start

Requires Docker and Docker Compose. Run commands from the repository root:

```bash
# On first setup only; keep an existing config.env.
cp example_config.env config.env
# Set your own JWT_SECRET_KEY in config.env before starting.
docker compose --env-file config.env up --build -d
```

Compose starts `db-currency-wallet`, applies migrations using `migrate-wallet`,
and starts the API after migrations succeed. No local Go or migrate installation
is required for this workflow. The full Compose stack also starts the exchanger database
and its migrations, as well as the exchanger application. The default API URL is `http://localhost:8080`.

Inspect startup status and logs:

```bash
docker compose --env-file config.env ps -a
docker compose --env-file config.env logs migrate-wallet wallet
```

Stop the application:

```bash
docker compose --env-file config.env down
```

PostgreSQL data persists in `out/wallet_pg_data`. Local configuration and database data
are excluded from Git. Database initialization settings apply to a new data directory;
changing credentials in the env file does not update an existing database user.

Make shortcuts:

```bash
make docker-start
make docker-stop
make docker-rebuild
```

`docker-rebuild` rebuilds both applications. To rebuild wallet only, run
`docker compose --env-file config.env up -d --build wallet`. Restarting an existing
container does not rebuild its image.

## Configuration

Set `JWT_SECRET_KEY` to a random secret of at least 32 bytes; `openssl rand -hex 32`
can generate one. Replace the placeholder in `config.env` and keep this file out
of Git. Changing the signing key invalidates previously issued tokens.


Copy `example_config.env` to `config.env` before starting the application.
Compose passes the file's variables to the API, which reads them from its environment.
For direct local execution, `-c config.env` reads the file without exporting it;
explicit environment variables override file values.

| Variable | Purpose | Example |
|---|---|---|
| `POSTGRES_USER` | Database user | `test_user` |
| `POSTGRES_PASSWORD` | Database password | `test_pass` |
| `POSTGRES_HOST` | Database host inside Compose | `db-currency-wallet` |
| `POSTGRES_NAME` | Database name | `bank` |
| `POSTGRES_PORT` | Published database port for local connections | `5432` |
| `HTTP_PORT` | API listening port, without a colon | `8080` |
| `HTTP_OUT_PORT` | Published API port on the host | `8080` |
| `JWT_SECRET_KEY` | Required HS256 signing secret, at least 32 bytes | Your own random secret |
| `EXCHANGER_GRPC_DOCKER_ADDR` | Exchanger address when running without `-c` | `exchanger:50051` |
| `EXCHANGER_GRPC_LOCALHOST_ADDR` | Exchanger address when loading configuration with `-c` | `localhost:50051` |
| `EXCHANGER_TIMEOUT` | Required positive integer timeout for each gRPC request, in seconds | `5` |
| `EXCHANGE_RATES_CACHE_TTL` | Required positive integer lifetime of the in-memory rates cache in seconds | `30` |
| `JWT_TTL` | Required positive integer lifetime in hours | `24` |
| `LOG_LEVEL_WALLET` | Optional JSON log level: DEBUG, INFO, WARN, ERROR | `INFO` |
| `MAX_DB_CONNECTIONS` | Maximum connections in the API database pool | `4` |
| `MIN_DB_CONNECTIONS` | Minimum connections maintained by the pool | `1` |
| `READ_HEADER_TIMEOUT` | HTTP request header read timeout, in seconds | `5` |
| `READ_TIMEOUT` | Entire HTTP request read timeout, in seconds | `5` |
| `WRITE_TIMEOUT` | HTTP response write timeout, in seconds | `20` |
| `IDLE_TIMEOUT` | HTTP keep-alive idle timeout, in seconds | `120` |

The API container overrides `POSTGRES_PORT` to `5432`, the PostgreSQL port inside
Compose. HTTP ports are mapped as `HTTP_OUT_PORT:HTTP_PORT`.

The application requires all settings above except `LOG_LEVEL_WALLET` and
`HTTP_OUT_PORT`; the latter is used only by Compose. Pool limits and timeouts must be
integers. `MAX_DB_CONNECTIONS` must be between 1 and 2147483647;
`MIN_DB_CONNECTIONS` must be between 0 and `MAX_DB_CONNECTIONS`, inclusive.
Timeouts must be positive and fit in a Go duration when converted from seconds
(at most 9223372036 seconds). These are validation limits, not recommended tuning values.
`JWT_TTL` accepts integer hours from 1 to 2562047; values such as `24h` or `1.5`
are rejected. Its conversion to a duration is checked for overflow.
The same positive-seconds duration limits apply to `EXCHANGER_TIMEOUT` and
`EXCHANGE_RATES_CACHE_TTL`. Both exchanger address variables are required.
Invalid pool or timeout settings cause startup to fail with the variable name in the error.
When updating an existing `config.env`, add missing settings from `example_config.env`.

To compare pool sizes, change `MAX_DB_CONNECTIONS` in `config.env`, keeping
`MIN_DB_CONNECTIONS` no greater than the maximum, and recreate the API container:

```bash
docker compose --env-file config.env up -d --no-deps --force-recreate wallet
```

An image rebuild is not needed for environment-only changes once the image includes
support for these settings. A plain container restart does not reload `config.env`.
Run identical load scenarios for each pool size and compare throughput, latency,
errors, and final balances. Increasing the pool size does not guarantee higher throughput
when all updates target the same user and currency. `WRITE_TIMEOUT` controls response writes;
it does not set a database query timeout.

## OpenAPI and Swagger

The [OpenAPI 3.0.3 specification](docs/openapi.yaml) describes all seven REST
endpoints, request and response schemas, error statuses and JWT bearer authentication.
The file is maintained manually; keep it in sync with HTTP handlers when the API changes.

After starting the application, open [Swagger UI](http://localhost:8080/swagger/).
The raw specification is available at [GET /openapi.yaml](http://localhost:8080/openapi.yaml).
Both documentation routes are public; API authentication is unchanged. HTML and
YAML are embedded in the Go binary, so edits require rebuilding and restarting the
application. Swagger UI loads JavaScript and CSS from a CDN; the browser needs
internet access.

Use **Try it out** on registration and login, then copy the login token into
**Authorize** without the `Bearer` prefix. Swagger UI adds the prefix to protected
requests automatically.

Its server URL is `http://localhost:8080`; adjust it in your client if using another
host or published port. Protected operations require a JWT obtained from
`POST /api/v1/login` and sent as `Authorization: Bearer <token>`.

## Authentication

`POST /api/v1/register` and `POST /api/v1/login` are public. Wallet, exchange-rate and currency-exchange
routes below require `Authorization: Bearer <token>`. Missing, invalid or expired
tokens receive `401 Unauthorized` with an empty body.

The user ID comes from the verified JWT. Requests operate only on that user's
balances; no wallet ID or user ID is needed in the request body. Registration
creates the user and zero USD, RUB and EUR balances in one transaction.

### Register

```bash
curl -i -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"ivan","email":"ivan@example.com","password":"example-password"}'
```

Success: `201 Created`, `{"message":"User registered successfully"}`.
Username is trimmed and lowercased. Email syntax is checked and email uniqueness
is case-insensitive. Passwords must be nonblank and at most 72 bytes, not characters;
spaces in an otherwise valid password are preserved. Passwords are stored as bcrypt hashes.
Duplicate username or email: `400`, `{"error":"Username or email already exists"}`.
Other invalid registration fields also return `400` with an `error` string.

### Log in

```bash
curl -i -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"ivan","password":"example-password"}'
```

Success: `200 OK`, `{"token":"<JWT>"}`. Copy the returned token for subsequent examples:

```bash
TOKEN='<JWT returned by login>'
```

Invalid credentials or invalid password: `401`,
`{"error":"Invalid username or password"}`. Unexpected internal errors return
`500`, `{"error":"internal server error"}`. Both auth endpoints accept one JSON
value with a 4096-byte limit: malformed input returns `400`, oversized input `413`.
Auth and wallet handler errors use `{"error":"..."}`.

JWTs use HS256 with `sub` (user ID), `iat` (issued at), and `exp` (expiry).
There is no refresh-token or individual-token revocation endpoint; log in again
when the token expires. Middleware verifies the signature, algorithm and expiry,
then passes a nonzero UUID user ID through the request context.

## API

| Method | Route | Purpose |
|---|---|---|
| POST | `/api/v1/register` | Create a user and three zero balances |
| POST | `/api/v1/login` | Receive a JWT |
| GET | `/api/v1/exchange/rates` | Read cached exchange rates relative to USD |
| POST | `/api/v1/exchange` | Exchange between two currency balances atomically |
| GET | `/api/v1/balance` | Read the authenticated user's balances |
| POST | `/api/v1/wallet/deposit` | Deposit into one currency balance |
| POST | `/api/v1/wallet/withdraw` | Withdraw from one currency balance |

All routes except registration and login require `Authorization: Bearer <token>`.
The old `/api/v1/wallets`, `/api/v1/wallets/{wallet_uuid}` and
`/api/v1/wallet` routes have been removed. There is no separate create-wallet endpoint.

### Exchange rates

```bash
curl -i http://localhost:8080/api/v1/exchange/rates \
  -H "Authorization: Bearer $TOKEN"
```

Success: `200 OK`; with the seeded exchanger database:

```json
{"rates":{"USD":1,"RUB":90.1,"EUR":0.87}}
```

Rates represent currency units per 1 USD. This endpoint only reads rates;
it does not convert money or change balances.

Each wallet process caches successful responses for `EXCHANGE_RATES_CACHE_TTL`
seconds, starting when the upstream response is received. A fresh cache can serve
requests while exchanger is unavailable. After expiry, the next request refreshes
rates over gRPC with `EXCHANGER_TIMEOUT`; concurrent requests share the cache
and serialize refresh attempts. There is no background refresh.

If the cache is empty or expired and exchanger fails, the endpoint returns
`500` with `{"error":"Failed to retrieve exchange rates"}`. Expired rates are
not returned, and errors do not extend the cache lifetime; the next request
tries again. Restarting wallet clears its cache.

### Money and currencies

API amounts use major currency units: `1` means 1.00 USD/EUR/RUB, and `0.01`
is the smallest supported amount. PostgreSQL stores integer minor units in `BIGINT`
(`1.23` becomes `123`); conversion does not use floating-point arithmetic.
Responses contain JSON numbers with two decimal places.

Currencies are exactly `USD`, `EUR` and `RUB` (uppercase). Operation amounts must
be positive, have at most two fractional digits, and fit into `int64` minor units.
Scientific notation such as `1e2` is rejected. The maximum amount or balance is
`92233720368547758.07` major units. Balances cannot be negative.
No currency conversion is performed by deposit or withdraw.
For EUR/RUB, both operations now require a rate from the shared cache or exchanger
before changing the balance, in preparation for classifying large operations.
If the cache is empty or expired and fetching rates fails, or the currency rate
is missing, the operation returns 500 without changing the balance. USD uses a
fixed rate of 1 and does not call the rates provider. Event publication is not
implemented yet.

### Read balances

```bash
curl -i http://localhost:8080/api/v1/balance \
  -H "Authorization: Bearer $TOKEN"
```

A newly registered user receives `200 OK`:

```json
{"balance":{"USD":0.00,"EUR":0.00,"RUB":0.00}}
```

Currency key order is not significant.

### Deposit

Deposit and withdrawal endpoints accept one JSON object, with a 4096-byte body limit.

```bash
curl -i -X POST http://localhost:8080/api/v1/wallet/deposit \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"currency":"USD","amount":100.00}'
```

For the new user above, success is `200 OK`:

```json
{"message":"Account topped up successfully","new_balance":{"USD":100.00,"EUR":0.00,"RUB":0.00}}
```

### Withdraw

```bash
curl -i -X POST http://localhost:8080/api/v1/wallet/withdraw \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"currency":"USD","amount":25.50}'
```

After the deposit above, success is `200 OK`:

```json
{"message":"Withdrawal successful","new_balance":{"USD":74.50,"EUR":0.00,"RUB":0.00}}
```

Withdrawing the entire balance is allowed. Each accepted deposit or withdrawal POST is a separate
operation; there is no idempotency key, so retries can apply it again.
For deposit and withdrawal, the SQL update is atomic. Reading `new_balance` is a separate query: its result
can include concurrent operations, and a read failure can return 500 after the
update has succeeded.

### Exchange currencies

`POST /api/v1/exchange` requires a JWT and one JSON object, limited to 4096 bytes.
Amounts are JSON numbers in major units; both currencies must be supported and
different. The debit must be positive and have at most two decimal places.

This example starts with 100.00 USD and 0.00 EUR and uses the seeded EUR rate of
0.87. If following the deposit and withdrawal examples above, first deposit another
25.50 USD to bring the remaining 74.50 USD back to 100.00 USD:

```bash
curl -i -X POST http://localhost:8080/api/v1/wallet/deposit \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"currency":"USD","amount":25.50}'
```

Then exchange the 100.00 USD:

```bash
curl -i -X POST http://localhost:8080/api/v1/exchange \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"from_currency":"USD","to_currency":"EUR","amount":100.00}'
```

Success: `200 OK`:

```json
{"message":"Exchange successful","exchanged_amount":87.00,"new_balance":{"USD":0.00,"EUR":87.00}}
```

`exchanged_amount` is the credited amount in the target currency. `new_balance`
contains only the two affected currencies and their balances from this transaction.
Other balances are unchanged. The result is returned after a successful commit;
concurrent later operations may change the balances before the response arrives.

Exchange uses the same process-local rates cache as `GET /api/v1/exchange/rates`.
A prior rates request is optional: an empty or expired cache triggers a gRPC
request. Rates are obtained before opening the database transaction. If refreshing
rates fails, the exchange fails without changing balances; a fresh cache can
serve an exchange while exchanger is unavailable.

Both balance rows are locked in currency-code order. Insufficient funds or credit
overflow aborts the transaction; a failure during credit rolls back the debit.
There is no idempotency key: repeating a successful request performs another exchange.

Exchange error responses use `{"error":"..."}`:

| HTTP status | Message | When |
|---|---|---|
| 400 | `Invalid request body` | Malformed JSON, incompatible field types or multiple JSON values |
| 400 | `Insufficient funds or invalid currencies` | Unsupported or identical currencies, invalid amount, insufficient funds, credit rounded to zero, or balance overflow |
| 413 | `Request body too large` | Body exceeds 4096 bytes |
| 500 | `Internal server error` | Rates unavailable or missing, a missing balance row, or an unexpected internal failure |

Missing, invalid or expired JWTs receive `401` with an empty body.


### Exchange rounding

Currency exchange credits are rounded down to whole minor units after computing
`amount * target_rate / source_rate`. Fractional minor units are discarded; a
result below one minor unit is rejected. At unchanged rates, rounding cannot
increase the balance through an exchange and its reverse. For example, with
RUB=90.1 and EUR=0.87 per USD, 11213.82 RUB becomes 108.27 EUR; exchanging it
back yields 11212.78 RUB.

## Error responses

Deposit and withdrawal handler errors use `{"error":"..."}`:

| HTTP status | Message | When |
|---|---|---|
| 400 | `invalid request body` | Malformed JSON or multiple JSON values |
| 400 | `Invalid amount or currency` | Invalid deposit amount or currency |
| 400 | `Insufficient funds or invalid amount` | Invalid withdrawal amount or insufficient funds |
| 400 | `invalid currency` | Unsupported withdrawal currency |
| 413 | `request body too large` | Request body exceeds 4096 bytes |
| 422 | `balance overflow` | Deposit would exceed the maximum balance |
| 500 | `internal server error` | Unexpected failure, including a missing balance row during an operation |

Missing, invalid or expired JWTs receive 401 with an empty body from middleware.
Unknown routes and unsupported HTTP methods use standard `net/http` responses.

## Migrations

Migrations run in order: `000001` creates the legacy wallets table, `000002`
creates users, and `000003` replaces wallets with currencies and per-user balances.
The balance primary key is `(user_id, currency)`; amounts must be nonnegative.

Migration `000003` deliberately deletes all old anonymous wallets and their money.
Rolling it back recreates an empty legacy table; it does not restore deleted data.
Keep the earlier migration files: a fresh database still applies the full sequence.

Existing users are not backfilled by `000003`. Until a backfill migration is added,
use a newly registered user for the balance API. An existing user with no balance
rows receives `{"balance":{}}` on GET; deposit/withdraw currently return 500.
New registrations create all three balances atomically and roll back if this fails.

## Local run

Requires a Go toolchain compatible with the root `go.work` and this service’s `go.mod`, plus Make. Start the database and
apply migrations, then run the API locally:

```bash
make db-wallet
make local-run-wallet
```

Ensure migrations succeeded before running the API. Stop any existing `wallet`
container if it occupies the local API port. The Make target loads `config.env`
and overrides `POSTGRES_HOST` to `localhost`. Locally the API listens on `HTTP_PORT`.

Alternatively, run from the repository root with file configuration:

```bash
POSTGRES_HOST=localhost go run ./services/gw-currency-wallet/cmd -c config.env
```

Without `-c`, the application reads only environment variables. The existing
`make local-run-go` target remains supported.

## Logging

The application writes JSON logs with `service=gw-currency-wallet`. Set
`LOG_LEVEL_WALLET` to `DEBUG`, `INFO` (default), `WARN` or `ERROR`.
Logs cover startup, shutdown, HTTP outcomes and successful balance changes.
HTTP records include method, route pattern, status and duration; request bodies,
query strings and authorization headers are not included. Startup failures go to
stderr; regular application logs go to stdout.

## Tests

Run unit tests without PostgreSQL:

```bash
make test-wallet
```

Repository integration tests are skipped unless `TEST_DATABASE_URL` is set.

Run integration tests with a temporary PostgreSQL instance:

```bash
make integration-test
```

Requires Go, Make, Docker Compose with `up --wait` support, and a running local
Docker daemon. The target covers wallet balance, auth, currency exchange and exchanger repositories. The command uses `docker-compose.test.yaml`, selects an available
local port, waits for database readiness, and runs the repository tests.
Containers, network, and volumes are removed after success, failure, or interruption.
Each PostgreSQL test applies its migrations in a separate schema and cleans it up afterward.
These tests do not use `config.env` or the development database.

Coverage includes registration/login, bcrypt, JWT validation, authentication middleware,
configuration, domain validation, exact decimal conversion and HTTP error responses.
Exchange-rate tests cover client validation, RPC deadlines, handler responses,
cache expiry and refresh failure, concurrent cache misses and cancellation.
Exchange tests cover calculation and rounding down, absence of rounding gains on
a round trip at unchanged rates, HTTP contracts, repository calls and errors.
PostgreSQL tests check both exchange directions, returned balances and credited
amount, user isolation, missing balances, full-balance exchange, rollback after a
credit failure, and concurrent exchanges without overdrafts or overflow.
Repository tests cover registration rollback, user/currency isolation, balance limits,
concurrent deposits and withdrawals, and prevention of overdrafts and overflow.

## Load testing

Requires Vegeta and a running API. Register a dedicated test user, log in, and set
`TOKEN` to the returned JWT as shown above. The committed target files contain no
credentials; pass authentication through Vegeta's header option.

From the repository root (1,000 requests/sec for 30 seconds):

```bash
export TOKEN
make load-test-add-balance
make load-test-get-balance
make load-test-minus-balance
```

The Make targets require a nonempty `TOKEN`. Both operation body files use
`{"currency":"USD","amount":0.01}`. A 30-second run submits about 30,000 operations,
so a withdrawal run needs at least 300.00 USD if every request succeeds.
Check the actual balance after deposits before starting withdrawals.

For a custom duration, run from `services/gw-currency-wallet/`:

```bash
vegeta attack -targets=./loadtests/vegeta_targets_add_balance.txt \
  -header="Authorization: Bearer $TOKEN" -rate=1000 -duration=60s | vegeta report
```

Adjust target URLs if the API uses a different address. A 60-second withdrawal
run with the supplied amount needs at least 600.00 USD. Ensure the JWT remains
valid throughout the run. Compare the final balance with the starting balance
and successful operations; separately inspect any failed responses.

Previous load measurements used the removed anonymous-wallet API and are not
benchmarks for the current authenticated multicurrency API. No new load results
are claimed here.
