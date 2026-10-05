# Changelog

## 0.4.1

- Đổi đường dẫn module thành `github.com/mona-software/monapay-go`. Đường cũ `github.com/themonagroup/monapay-go` không tải được nữa vì org cũ đã bị GitHub khoá. Code đang dùng phải đổi import và chạy `go get github.com/mona-software/monapay-go`.

## 0.4.0

- Thêm `PaymentProfile`, `Checkouts`, xem lại/xoay secret hồ sơ và API key.
- Tự sinh `Idempotency-Key` cho tạo/huỷ checkout và cho phép truyền key riêng.

## 0.3.0

- Client credentials mặc định, cache token theo hạn và hỗ trợ `NewClientFromEnv`.
- Thêm sandbox transactions, email configs, email logs và email suppressions.

## 0.1.0

- Bản đầu tiên của SDK Go.
