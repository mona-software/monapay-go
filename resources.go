package monapay

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type KeysResource struct{ client *Client }

func (r *KeysResource) Generate(ctx context.Context, name string) (any, error) {
	if name == "" {
		name = "Default Key"
	}
	data, err := r.client.request(ctx, http.MethodPost, "/api/v1/client-keys/generate", map[string]any{"name": name}, nil)
	if err == nil {
		if object, ok := data.(map[string]any); ok {
			if secret, ok := object["client_secret"].(string); ok && secret != "" {
				_, current := r.client.getAuth()
				if current == "" {
					r.client.SetClientSecret(secret)
				}
			}
		}
	}
	return data, err
}

func (r *KeysResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client-keys/list", nil, nil)
}

func (r *KeysResource) Destroy(ctx context.Context, keyID string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/client-keys/destroy/"+segment(keyID), nil, nil)
}

func (r *KeysResource) Reveal(ctx context.Context, keyID string, confirmation map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/client-keys/"+segment(keyID)+"/reveal", confirmation, nil)
}

func (r *KeysResource) Rotate(ctx context.Context, keyID string) (any, error) {
	data, err := r.client.request(ctx, http.MethodPost, "/api/v1/client-keys/"+segment(keyID)+"/rotate", map[string]any{}, nil)
	if err == nil {
		if object, ok := data.(map[string]any); ok {
			if secret, ok := object["client_secret"].(string); ok && secret != "" {
				r.client.SetClientSecret(secret)
			}
		}
	}
	return data, err
}

type VirtualAccountsResource struct{ client *Client }

func (r *VirtualAccountsResource) Register(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/virtual-account/registration", body, nil)
}

func (r *VirtualAccountsResource) Verify(ctx context.Context, requestID, code string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(requestID)+"/virtual-account/verification", map[string]any{"code": code}, nil)
}

func (r *VirtualAccountsResource) RegisterNotification(ctx context.Context, vaID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(vaID)+"/notification/registration", body, nil)
}

func (r *VirtualAccountsResource) VerifyNotification(ctx context.Context, requestID, code string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(requestID)+"/notification/verification", map[string]any{"code": code}, nil)
}

func (r *VirtualAccountsResource) List(ctx context.Context, bankAccountID string) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/acb/"+segment(bankAccountID)+"/virtual-account/retrieve", nil, nil)
}

type BankAccountsResource struct{ client *Client }

func (r *BankAccountsResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client/bank-accounts", nil, nil)
}

type PaymentProfileResource struct{ client *Client }

func (r *PaymentProfileResource) Get(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/payment-profile", nil, nil)
}

func (r *PaymentProfileResource) Set(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPut, "/api/v1/payment-profile", body, nil)
}

func (r *PaymentProfileResource) RotateReturnSecret(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/payment-profile/rotate-return-secret", map[string]any{}, nil)
}

func (r *PaymentProfileResource) RevealReturnSecret(ctx context.Context, confirmation map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/payment-profile/reveal-return-secret", confirmation, nil)
}

type CheckoutOptions struct {
	Status, OrderCode, FromDate, ToDate string
	Page, Limit                         int
}

type CheckoutsResource struct{ client *Client }

func (r *CheckoutsResource) Create(ctx context.Context, body map[string]any, key string) (any, error) {
	key, err := idempotencyKey(key)
	if err != nil {
		return nil, err
	}
	return r.client.requestWithHeaders(ctx, http.MethodPost, "/api/v1/checkouts", body, nil, http.Header{"Idempotency-Key": {key}})
}

func (r *CheckoutsResource) Get(ctx context.Context, checkoutID string) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/checkouts/"+segment(checkoutID), nil, nil)
}

func (r *CheckoutsResource) List(ctx context.Context, options CheckoutOptions) (any, error) {
	query := url.Values{}
	values := map[string]string{"status": options.Status, "order_code": options.OrderCode, "from_date": options.FromDate, "to_date": options.ToDate}
	for name, value := range values {
		if value != "" {
			query.Set(name, value)
		}
	}
	if options.Page > 0 {
		query.Set("page", fmt.Sprint(options.Page))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprint(options.Limit))
	}
	return r.client.request(ctx, http.MethodGet, "/api/v1/checkouts", nil, query)
}

func (r *CheckoutsResource) Cancel(ctx context.Context, checkoutID, key string) (any, error) {
	key, err := idempotencyKey(key)
	if err != nil {
		return nil, err
	}
	return r.client.requestWithHeaders(ctx, http.MethodPost, "/api/v1/checkouts/"+segment(checkoutID)+"/cancel", map[string]any{}, nil, http.Header{"Idempotency-Key": {key}})
}

type QRResource struct{ client *Client }

func (r *QRResource) Generate(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/qr-payment/generate", body, nil)
}

func (r *QRResource) Cancel(ctx context.Context, qrCodeID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/acb/qr-payment/"+segment(qrCodeID)+"/cancellation", body, nil)
}

type TransactionsResource struct{ client *Client }

type TransactionOptions struct {
	Page    int
	Limit   int
	SinceID string
}

func (r *TransactionsResource) List(ctx context.Context, virtualAccountNumber string, options TransactionOptions) (any, error) {
	if virtualAccountNumber == "" {
		return nil, fmt.Errorf("virtualAccountNumber là bắt buộc")
	}
	query := url.Values{}
	query.Set("virtual_account_number", virtualAccountNumber)
	query.Set("page", fmt.Sprint(positive(options.Page, 1)))
	query.Set("limit", fmt.Sprint(positive(options.Limit, 100)))
	// since_id is intentionally client-side: the current backend does not accept it.
	return r.client.request(ctx, http.MethodGet, "/api/v1/acb/virtual-account/transactions", nil, query)
}

func (r *TransactionsResource) Iterate(ctx context.Context, virtualAccountNumber string, options TransactionOptions) *TransactionIterator {
	return &TransactionIterator{
		ctx:      ctx,
		resource: r,
		vaNumber: virtualAccountNumber,
		page:     positive(options.Page, 1),
		limit:    positive(options.Limit, 100),
		sinceID:  options.SinceID,
	}
}

func (r *TransactionsResource) Retry(ctx context.Context, transactionID, targetType, targetID string) (any, error) {
	body := map[string]any{"target_type": targetType}
	if targetID != "" {
		body["target_id"] = targetID
	}
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/virtual-account/transactions/"+segment(transactionID)+"/retry", body, nil)
}

// TransactionIterator lazily retrieves transaction pages. Value is valid after Next returns true.
type TransactionIterator struct {
	ctx      context.Context
	resource *TransactionsResource
	vaNumber string
	page     int
	limit    int
	sinceID  string
	items    []any
	index    int
	current  any
	done     bool
	err      error
}

func (i *TransactionIterator) Next() bool {
	for {
		if i.index < len(i.items) {
			i.current = i.items[i.index]
			i.index++
			return true
		}
		if i.done || i.err != nil {
			return false
		}
		data, err := i.resource.List(i.ctx, i.vaNumber, TransactionOptions{Page: i.page, Limit: i.limit})
		if err != nil {
			i.err = err
			return false
		}
		page, ok := data.(map[string]any)
		if !ok {
			i.err = fmt.Errorf("response giao dịch không phải object")
			return false
		}
		i.items = i.items[:0]
		i.index = 0
		if rawItems, ok := page["data"].([]any); ok {
			for _, item := range rawItems {
				if i.sinceID != "" && transactionMatches(item, i.sinceID) {
					i.done = true
					break
				}
				i.items = append(i.items, item)
			}
		}
		if !i.done {
			hasNext, known := page["has_next"].(bool)
			if known {
				i.done = !hasNext
			} else {
				i.done = i.page >= intValue(page["last_page"], i.page)
			}
		}
		i.page++
	}
}

func (i *TransactionIterator) Value() any { return i.current }
func (i *TransactionIterator) Err() error { return i.err }

func transactionMatches(item any, sinceID string) bool {
	object, ok := item.(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(object["id"]) == sinceID || fmt.Sprint(object["transaction_code"]) == sinceID
}

type WebhooksResource struct{ client *Client }

func (r *WebhooksResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client-webhooks", nil, nil)
}
func (r *WebhooksResource) Create(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/client-webhooks", body, nil)
}
func (r *WebhooksResource) Update(ctx context.Context, configID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPut, "/api/v1/client-webhooks/"+segment(configID), body, nil)
}
func (r *WebhooksResource) Remove(ctx context.Context, configID string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/client-webhooks/"+segment(configID), nil, nil)
}
func (r *WebhooksResource) Test(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/client-webhooks/test", body, nil)
}

type WebhookLogsResource struct{ client *Client }

type WebhookLogOptions struct {
	Status   string
	FromDate string
	ToDate   string
	Page     int
	Limit    int
}

func webhookLogQuery(options WebhookLogOptions) url.Values {
	query := url.Values{}
	if options.Status != "" {
		query.Set("status", options.Status)
	}
	if options.FromDate != "" {
		query.Set("from_date", options.FromDate)
	}
	if options.ToDate != "" {
		query.Set("to_date", options.ToDate)
	}
	if options.Page > 0 {
		query.Set("page", fmt.Sprint(options.Page))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprint(options.Limit))
	}
	return query
}

func (r *WebhookLogsResource) List(ctx context.Context, options WebhookLogOptions) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/webhook-logs", nil, webhookLogQuery(options))
}
func (r *WebhookLogsResource) Stats(ctx context.Context, options WebhookLogOptions) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/webhook-logs/stats", nil, webhookLogQuery(options))
}

type SandboxResource struct{ client *Client }

func (r *SandboxResource) CreateTransaction(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/sandbox/transactions", body, nil)
}

type EmailConfigsResource struct{ client *Client }

func (r *EmailConfigsResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/email-configs", nil, nil)
}
func (r *EmailConfigsResource) Create(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/email-configs", body, nil)
}
func (r *EmailConfigsResource) Get(ctx context.Context, configID string) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/email-configs/"+segment(configID), nil, nil)
}
func (r *EmailConfigsResource) Update(ctx context.Context, configID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPut, "/api/v1/email-configs/"+segment(configID), body, nil)
}
func (r *EmailConfigsResource) Remove(ctx context.Context, configID string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/email-configs/"+segment(configID), nil, nil)
}
func (r *EmailConfigsResource) Verify(ctx context.Context, configID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/email-configs/"+segment(configID)+"/verify", body, nil)
}
func (r *EmailConfigsResource) ResendVerification(ctx context.Context, configID, email string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/email-configs/"+segment(configID)+"/resend-verification", map[string]any{"email": email}, nil)
}
func (r *EmailConfigsResource) Test(ctx context.Context, configID string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/email-configs/"+segment(configID)+"/test", map[string]any{}, nil)
}

type EmailLogOptions struct {
	ConfigID, Status, EventType, FromDate, ToDate string
	Page, Limit                                   int
}

func emailLogQuery(options EmailLogOptions) url.Values {
	query := url.Values{}
	values := map[string]string{"config_id": options.ConfigID, "status": options.Status, "event_type": options.EventType, "from_date": options.FromDate, "to_date": options.ToDate}
	for key, value := range values {
		if value != "" {
			query.Set(key, value)
		}
	}
	if options.Page > 0 {
		query.Set("page", fmt.Sprint(options.Page))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprint(options.Limit))
	}
	return query
}

type EmailLogsResource struct{ client *Client }

func (r *EmailLogsResource) List(ctx context.Context, options EmailLogOptions) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/email-logs", nil, emailLogQuery(options))
}
func (r *EmailLogsResource) Stats(ctx context.Context, options EmailLogOptions) (any, error) {
	query := url.Values{}
	if options.FromDate != "" {
		query.Set("from_date", options.FromDate)
	}
	if options.ToDate != "" {
		query.Set("to_date", options.ToDate)
	}
	return r.client.request(ctx, http.MethodGet, "/api/v1/email-logs/stats", nil, query)
}

type EmailSuppressionsResource struct{ client *Client }

func (r *EmailSuppressionsResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/email-suppressions", nil, nil)
}
func (r *EmailSuppressionsResource) Remove(ctx context.Context, email string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/email-suppressions/"+segment(email), nil, nil)
}
