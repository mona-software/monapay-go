# MONA Pay SDK for Go

SDK Go 1.21+, zero-dependency cho MONA Pay, cổng thanh toán và API ngân hàng của The MONA Group. Tiền chuyển thẳng vào tài khoản doanh nghiệp; SDK hỗ trợ VA, VietQR, webhook và Telegram.

## Xác thực cho AI agent

```bash
export MONAPAY_CLIENT_ID="client-id"
export MONAPAY_CLIENT_SECRET="client-secret"
export MONAPAY_BASE_URL="https://api.monapay.vn"
```

```go
client, err := monapay.NewClientFromEnv()
if err != nil { log.Fatal(err) }
profile, err := client.Me(context.Background())
qr, err := client.QR.Generate(context.Background(), qrBody)
sandbox, err := client.Sandbox.CreateTransaction(context.Background(), map[string]any{"virtual_account_number": "MONA123", "amount": 10000, "description": "AI test"})
```

`NewClientFromEnv` dùng client credentials, cache token tới gần hạn và tự lấy lại token một lần khi gặp HTTP 401. Username/password chỉ là fallback tương thích cũ, không dùng cho AI agent vì sẽ gãy khi bật 2FA.

## Cài đặt

```bash
go get github.com/themonagroup/monapay-go@v0.3.0
```

## Dùng nhanh

```go
client, err := monapay.NewClient(monapay.Config{
    Username: os.Getenv("MONA_USERNAME"),
    Password: os.Getenv("MONA_PASSWORD"),
    ClientSecret: os.Getenv("MONA_CLIENT_SECRET"),
})
if err != nil { log.Fatal(err) }

profile, err := client.Me(context.Background())
hooks, err := client.Webhooks.List(context.Background())
```

Các resource: `Keys`, `BankAccounts`, `VA` (đăng ký + hai bước OTP), `QR`, `Transactions`, `Webhooks`, `WebhookLogs`, `Sandbox`, `EmailConfigs`, `EmailLogs`, `EmailSuppressions`. Client cache Bearer token theo hạn, lấy lại đúng một lần khi gặp HTTP 401 và tự gắn `X-Client-Secret` cho POST/PUT/DELETE.

Đọc giao dịch mới kể từ mốc đã lưu:

```go
it := client.Transactions.Iterate(ctx, "MONA000001", monapay.TransactionOptions{
    Limit: 100,
    SinceID: "FT26240001234",
})
for it.Next() {
    transaction := it.Value().(map[string]any)
    // Lưu transaction_code làm idempotency key.
}
if err := it.Err(); err != nil { log.Fatal(err) }
```

`SinceID` là helper phía SDK: iterator dừng trước item có `id` hoặc `transaction_code` trùng mốc. SDK không gửi query `since_id`, vì API hiện chưa hỗ trợ tham số đó.

Xác thực webhook trên raw bytes trước khi parse JSON:

```go
result, err := monapay.VerifyWebhook(rawBody, timestampHeader, signatureHeader, secret)
if err != nil || !result.OK { /* trả 401 */ }
```

Ví dụ server đầy đủ: `examples/webhook/main.go`. Chạy test offline bằng `go test ./...`.

Docs: https://monapay.vn/docs · Hotline 1900 636 648 · info@themona.global. MONA Pay miễn phí hoàn toàn.

## English

Zero-dependency Go 1.21+ SDK for MONA Pay. It includes automatic login/token caching, one 401 refresh, virtual accounts and both OTP steps, VietQR, paginated transaction iteration with a client-side `SinceID` checkpoint, webhook configuration/logs/retry, and constant-time webhook verification. See the example and API above.

MIT © The MONA Group.
