# monapay-go

Go SDK for the MONA Pay API: create checkout links and VietQR codes, manage virtual accounts, webhooks and email notifications, and verify signed webhooks.

Requires Go 1.21+. Standard library only. The client is safe for concurrent use.

## Install

```bash
go get github.com/mona-software/monapay-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	monapay "github.com/mona-software/monapay-go"
)

func main() {
	client, err := monapay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	checkout, err := client.Checkouts.Create(context.Background(), map[string]any{
		"amount":     250000,
		"order_code": "DH10234",
		"return_url": "https://shop.example/payment/return",
	}, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(checkout.(map[string]any)["checkout_url"])
}
```

## Usage

### Client

```go
// From environment variables (see Configuration)
client, err := monapay.NewClientFromEnv()

// Or explicitly
client, err := monapay.NewClient(monapay.Config{
	ClientID:     os.Getenv("MONAPAY_CLIENT_ID"),
	ClientSecret: os.Getenv("MONAPAY_CLIENT_SECRET"),
	// BaseURL and HTTPClient are optional
})

profile, err := client.Me(ctx)
```

- Client credentials (`ClientID` + `ClientSecret`) are the recommended login. `Username`/`Password` is a legacy fallback and does not work for accounts with 2FA enabled.
- The Bearer token is cached until one minute before `expires_in`. A request that fails with HTTP 401 refreshes the token and is retried once.
- `X-Client-Secret` is sent on non-GET requests when a client secret is set.
- Methods return `(any, error)`; error responses are returned as `*monapay.APIError` with `Status` and `Body`.

### Resources

| Field | Methods |
| --- | --- |
| `Keys` | `Generate`, `List`, `Destroy`, `Reveal`, `Rotate` |
| `BankAccounts` | `List` |
| `VA` | `Register`, `Verify`, `RegisterNotification`, `VerifyNotification`, `List` |
| `PaymentProfile` | `Get`, `Set`, `RotateReturnSecret`, `RevealReturnSecret` |
| `Checkouts` | `Create`, `Get`, `List`, `Cancel` |
| `QR` | `Generate`, `Cancel` |
| `Transactions` | `List`, `Iterate`, `Retry` |
| `Sandbox` | `CreateTransaction` |
| `Webhooks` | `List`, `Create`, `Update`, `Remove`, `Test` |
| `WebhookLogs` | `List`, `Stats` |
| `EmailConfigs` | `List`, `Create`, `Get`, `Update`, `Remove`, `Verify`, `ResendVerification`, `Test` |
| `EmailLogs` | `List`, `Stats` |
| `EmailSuppressions` | `List`, `Remove` |

### Hosted checkout

`Checkouts.Create` and `Checkouts.Cancel` take an idempotency key as the last argument. Pass `""` to let the SDK generate one.

Fulfil orders from the `CHECKOUT_PAID` webhook or from `Checkouts.Get`, not from the browser redirect.

### Sandbox

```go
tx, err := client.Sandbox.CreateTransaction(ctx, map[string]any{
	"virtual_account_number": "MONA123",
	"amount":                 10000,
	"description":            "Sandbox test",
})
```

### Transactions

```go
it := client.Transactions.Iterate(ctx, "MONA000001", monapay.TransactionOptions{
	Limit:   100,
	SinceID: "FT26240001234",
})
for it.Next() {
	transaction := it.Value().(map[string]any)
	// Use transaction_code as the idempotency key when storing.
	_ = transaction
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

`SinceID` is handled by the SDK: the iterator stops before the item whose `id` or `transaction_code` matches. It is not sent to the API.

### Webhooks

Verify the raw request bytes before parsing JSON. `VerifyWebhook` takes the `X-Mona-Timestamp` and `X-Mona-Signature` header values and an optional tolerance in seconds (default 300).

```go
rawBody, _ := io.ReadAll(r.Body)
result, err := monapay.VerifyWebhook(rawBody,
	r.Header.Get("X-Mona-Timestamp"), r.Header.Get("X-Mona-Signature"), secret)
if err != nil || !result.OK {
	http.Error(w, "invalid signature", http.StatusUnauthorized)
	return
}
payload, _ := result.Payload.(map[string]any)
```

A complete server is in `examples/webhook/main.go`.

## Configuration

`NewClientFromEnv` reads:

| Variable | Purpose |
| --- | --- |
| `MONAPAY_CLIENT_ID`, `MONAPAY_CLIENT_SECRET` | API key credentials (recommended) |
| `MONAPAY_USERNAME`, `MONAPAY_PASSWORD` | Legacy password login; does not work for accounts with 2FA enabled |
| `MONAPAY_BASE_URL` | API base URL, defaults to `https://api.monapay.vn` |

The webhook example reads `MONA_WEBHOOK_SECRET`.

Documentation: https://monapay.vn/docs

## Development

```bash
go test ./...
```

## License

MIT

**MONA Pay is part of MONA Cloud by The MONA Group.**
