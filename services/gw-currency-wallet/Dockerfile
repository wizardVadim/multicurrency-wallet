FROM golang:1.27.1-bookworm

WORKDIR /app/services/gw-currency-wallet

COPY contracts/ /app/contracts/
COPY services/gw-currency-wallet/go.mod services/gw-currency-wallet/go.sum ./
RUN go mod download

COPY services/gw-currency-wallet/ ./
RUN go build -o /app/exe ./cmd

CMD ["/app/exe"]
